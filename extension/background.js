// Service worker. Captures requests from any number of tabs (each enabled via
// the popup) and forwards them to the JSON Inspector app over a WebSocket.
// Capture state is shown by swapping the toolbar icon and tooltip per tab: a
// red dot while the app is connected, an orange dot while the app is
// unreachable. A silent /health check runs before each connection attempt so a
// closed app never logs WebSocket errors, and the interceptor is re-injected on
// navigation.
const DEFAULT_PORT = 38761;
const KEEPALIVE = 'ji-keepalive';
const RECONNECT_MS = 2500;

const NORMAL_ICON = {
  16: 'icons/icon16.png',
  32: 'icons/icon32.png',
  48: 'icons/icon48.png',
  128: 'icons/icon128.png',
};
const RECORDING_ICON = {
  16: 'icons/recording-icon16.png',
  32: 'icons/recording-icon32.png',
  48: 'icons/recording-icon48.png',
  128: 'icons/recording-icon128.png',
};
const DISCONNECTED_ICON = {
  16: 'icons/disconnected-icon16.png',
  32: 'icons/disconnected-icon32.png',
  48: 'icons/disconnected-icon48.png',
  128: 'icons/disconnected-icon128.png',
};

// Everything the settings screen can change. `xhrOnly` is the one that affects
// the capture path, so it is read synchronously from this module-level copy.
const DEFAULT_SETTINGS = {
  rememberTabs: true,
  xhrOnly: true,
  clearOnExit: false,
};

let socket = null;
let reconnectTimer = null;
let pending = [];
let captureTabIds = new Set(); // tabs currently being captured
let captureTabMeta = new Map(); // tabId -> { title }
let capturedCounts = new Map(); // tabId -> number of captured requests
let capturedLastAt = new Map(); // tabId -> timestamp of the last one
let settings = { ...DEFAULT_SETTINGS };
let connected = false;

// Read once when the service worker spins up, then kept in step by the storage
// listener below — the capture handler can't await a storage read per request.
chrome.storage.local.get('settings').then(({ settings: stored }) => {
  settings = { ...DEFAULT_SETTINGS, ...(stored || {}) };
});

chrome.storage.onChanged.addListener((changes, area) => {
  if (area === 'local' && changes.settings) {
    settings = { ...DEFAULT_SETTINGS, ...(changes.settings.newValue || {}) };
  }
});

async function readState() {
  const sess = await chrome.storage.session.get('captureTabIds');
  const local = await chrome.storage.local.get({ port: DEFAULT_PORT, settings: DEFAULT_SETTINGS });
  settings = { ...DEFAULT_SETTINGS, ...(local.settings || {}) };
  return { captureTabIds: sess.captureTabIds ?? [], port: local.port || DEFAULT_PORT };
}

async function persistCapture() {
  await chrome.storage.session.set({ captureTabIds: Array.from(captureTabIds) });
}

function setCaptureIcon(tabId, state) {
  const path = state === 'recording' ? RECORDING_ICON : state === 'disconnected' ? DISCONNECTED_ICON : NORMAL_ICON;
  chrome.action.setIcon({ path, tabId });
}

function setCaptureTitle(tabId, title) {
  chrome.action.setTitle({ title, tabId });
}

function pluralRequests(n) {
  const m10 = n % 10;
  const m100 = n % 100;
  if (m10 === 1 && m100 !== 11) return n + ' запрос';
  if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return n + ' запроса';
  return n + ' запросов';
}

function captureTitleText(tabId) {
  const meta = captureTabMeta.get(tabId);
  const label = meta && meta.title ? '«' + meta.title + '»' : 'вкладка';
  return 'Перехват: ' + label + ' · ' + pluralRequests(capturedCounts.get(tabId) || 0);
}

function refreshIndicator() {
  for (const id of captureTabIds) {
    setCaptureIcon(id, connected ? 'recording' : 'disconnected');
    setCaptureTitle(id, connected ? captureTitleText(id) : 'Перехват: приложение недоступно');
  }
}

async function isAppRunning(port) {
  try {
    const resp = await fetch(`http://127.0.0.1:${port}/health`, { signal: AbortSignal.timeout(2000) });
    return resp.ok;
  } catch (_) {
    return false;
  }
}

async function connect(port) {
  clearTimeout(reconnectTimer);
  // Already talking to the app: re-dialling would drop a working socket only to
  // re-check the health probe, and a failed probe at that moment would leave
  // nothing connected at all.
  if (socket && socket.readyState === WebSocket.OPEN) {
    connected = true;
    return;
  }
  if (socket) {
    try { socket.close(); } catch (_) {}
    socket = null;
  }

  // Pre-check so a closed app never produces WebSocket connection errors.
  const running = await isAppRunning(port);
  if (!running) {
    connected = false;
    refreshIndicator();
    scheduleReconnect();
    return;
  }

  let ws;
  try {
    ws = new WebSocket(`ws://127.0.0.1:${port}`);
  } catch (_) {
    scheduleReconnect();
    return;
  }
  socket = ws;

  ws.onopen = () => {
    connected = true;
    refreshIndicator();
    sendState();
    const queue = pending.splice(0, pending.length);
    for (const msg of queue) {
      if (ws.readyState === WebSocket.OPEN) ws.send(msg);
    }
  };
  ws.onclose = () => {
    if (socket === ws) socket = null;
    connected = false;
    scheduleReconnect();
  };
  ws.onerror = () => {
    // onclose fires next and drives the reconnect; nothing to do here.
  };
  ws.onmessage = (event) => {
    let msg;
    try { msg = JSON.parse(event.data); } catch (_) { return; }
    // The app's "Приостановить перехват" — stop every tab's interceptor.
    if (msg.type === 'pause') {
      pauseAll();
    } else if (msg.type === 'resume') {
      resumeAll();
    }
  };
}

function scheduleReconnect() {
  clearTimeout(reconnectTimer);
  connected = false;
  refreshIndicator();
  reconnectTimer = setTimeout(async () => {
    const { port } = await readState();
    // A paused extension still needs its connection: that socket is the only
    // way the app can ask to resume.
    if (captureTabIds.size > 0 || paused) connect(port);
  }, RECONNECT_MS);
}

function disconnect() {
  clearTimeout(reconnectTimer);
  if (socket) {
    try { socket.close(); } catch (_) {}
    socket = null;
  }
  connected = false;
  pending = [];
}

function enqueue(payload) {
  const msg = JSON.stringify(payload);
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(msg);
  } else {
    pending.push(msg);
    if (pending.length > 200) pending.shift();
  }
}

// Reports the live capture status (recording flag + tab count) to the app, so
// its status bar reflects reality rather than inferring from badge counts.
function sendState() {
  if (!socket || socket.readyState !== WebSocket.OPEN) return;
  socket.send(JSON.stringify({
    type: 'state',
    recording: connected && captureTabIds.size > 0,
    tabs: captureTabIds.size,
    browser: 'Chrome',
  }));
}

async function injectInto(tabId) {
  await chrome.scripting.executeScript({ target: { tabId }, files: ['content/bridge.js'] });
  await chrome.scripting.executeScript({
    target: { tabId },
    files: ['content/interceptor.js'],
    world: 'MAIN',
    injectImmediately: true,
  });
}

async function disableInTab(tabId) {
  try {
    await chrome.scripting.executeScript({
      target: { tabId },
      func: () => { window.__jsonInspectorEnabled = false; },
      world: 'MAIN',
    });
  } catch (_) {}
}

async function startCapture() {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (!tab || tab.id == null) {
    return { ok: false, error: 'нет активной вкладки' };
  }

  // Starting a capture by hand is also a way out of the paused state.
  paused = false;

  if (!captureTabIds.has(tab.id)) {
    await injectInto(tab.id);
    captureTabIds.add(tab.id);
    captureTabMeta.set(tab.id, { title: tab.title || '', url: tab.url || '' });
    capturedCounts.set(tab.id, 0);
    if (settings.rememberTabs) await rememberOrigin(await originOf(tab.url));
    await persistCapture();
  }

  chrome.alarms.create(KEEPALIVE, { periodInMinutes: 0.4 });
  setCaptureIcon(tab.id, 'disconnected');
  setCaptureTitle(tab.id, 'Перехват: приложение недоступно');

  if (!(socket && socket.readyState === WebSocket.OPEN)) {
    const { port } = await readState();
    connect(port);
  } else {
    refreshIndicator();
    sendState();
  }
  return { ok: true, tabId: tab.id };
}

// `keepConnection` is for the pause: the app's "Возобновить перехват" arrives
// over this very socket, so closing it would leave the button with nothing to
// talk to. A stopped-by-hand tab still says goodbye as before.
async function stopCaptureFor(tabId, keepConnection = false) {
  await disableInTab(tabId);
  setCaptureIcon(tabId, 'normal');
  setCaptureTitle(tabId, 'JSON Inspector');
  // Forget the origin only when no other captured tab is still on it —
  // otherwise stopping one tab would silently drop the memory for the rest.
  // A pause keeps them too: they are what the resume restores.
  const forgotten = captureTabMeta.get(tabId);
  captureTabIds.delete(tabId);
  captureTabMeta.delete(tabId);
  capturedCounts.delete(tabId);
  capturedLastAt.delete(tabId);
  if (!keepConnection && forgotten && forgotten.url) {
    const origin = await originOf(forgotten.url);
    const stillUsed = await Promise.all(
      Array.from(captureTabIds).map(async (id) => {
        try {
          const t = await chrome.tabs.get(id);
          return (await originOf(t.url)) === origin;
        } catch (_) {
          return false;
        }
      })
    );
    if (!stillUsed.some(Boolean)) await forgetOrigin(origin);
  }
  await persistCapture();
  if (captureTabIds.size === 0 && !keepConnection) {
    chrome.alarms.clear(KEEPALIVE);
    disconnect();
  }
  sendState();
}

async function stopCapture() {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (!tab || tab.id == null) return { ok: true };
  await stopCaptureFor(tab.id);
  return { ok: true };
}

// What "Возобновить перехват" should put back. Session-only: stopping a tab
// forgets its origin on purpose (that's what makes "не помнить" work), so the
// paused set has to be kept separately to be resumable.
let pausedOrigins = [];
// While paused the connection is deliberately kept open: the app's resume
// command arrives over that same socket, and a closed one would make the button
// dead (there is no other channel back into the extension).
let paused = false;

// Re-injects into every open tab on one of these origins. Shared by the two
// things that restore capture without the user clicking a tab: resuming after a
// pause, and the browser restart when "Помнить вкладки" is on.
async function captureByOrigins(origins) {
  if (!origins.length) return 0;
  const tabs = await chrome.tabs.query({});
  for (const tab of tabs) {
    if (tab.id == null || !tab.url || captureTabIds.has(tab.id)) continue;
    if (!origins.includes(await originOf(tab.url))) continue;
    try {
      await injectInto(tab.id);
    } catch (_) {
      continue; // restricted page (chrome://, the web store) — skip it
    }
    captureTabIds.add(tab.id);
    captureTabMeta.set(tab.id, { title: tab.title || '', url: tab.url || '' });
    capturedCounts.set(tab.id, 0);
    if (settings.rememberTabs) await rememberOrigin(await originOf(tab.url));
  }
  if (captureTabIds.size === 0) return 0;
  await persistCapture();
  chrome.alarms.create(KEEPALIVE, { periodInMinutes: 0.4 });
  refreshIndicator();
  const { port } = await readState();
  await connect(port);
  // Reported explicitly: when a socket is already open `connect` returns
  // without re-opening, so `onopen` won't fire and the app would never hear
  // that recording is back on.
  sendState();
  return captureTabIds.size;
}

// Stops capture on every tab — the app's "Приостановить перехват" control.
async function pauseAll() {
  const origins = [];
  for (const id of Array.from(captureTabIds)) {
    const meta = captureTabMeta.get(id);
    const origin = meta ? await originOf(meta.url) : '';
    if (origin && !origins.includes(origin)) origins.push(origin);
  }
  pausedOrigins = origins;
  paused = true;
  // Kept where a restarted service worker can still find them: without this,
  // the resume would have no origins to restore and would silently do nothing.
  await chrome.storage.session.set({ pausedOrigins: origins });

  for (const id of Array.from(captureTabIds)) {
    await stopCaptureFor(id, true);
  }

  // The socket was left open by `keepConnection`; the alarm is what keeps the
  // worker (and with it the socket) alive while nothing is being recorded.
  chrome.alarms.create(KEEPALIVE, { periodInMinutes: 0.4 });
  const { port } = await readState();
  if (!(socket && socket.readyState === WebSocket.OPEN)) connect(port);
  sendState();
}

// The other half of that control: pick the paused tabs back up. Falls back to
// the remembered origins, so resuming still works after the worker was
// restarted and lost the in-memory list.
async function resumeAll() {
  const session = await chrome.storage.session.get('pausedOrigins');
  const remembered = await chrome.storage.local.get('captureOrigins');
  const origins = pausedOrigins.length
    ? pausedOrigins
    : session.pausedOrigins?.length
      ? session.pausedOrigins
      : remembered.captureOrigins || [];

  pausedOrigins = [];
  paused = false;
  await chrome.storage.session.set({ pausedOrigins: [] });
  await captureByOrigins(origins);
}

function makeId() {
  return Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 8);
}

// --- xhrOnly ---------------------------------------------------------------
// The hook only ever sees fetch/XHR, so "skip pictures, styles and fonts" means
// skipping the ones loaded *through* those APIs — a beacon misused as an image
// loader, an SPA fetching a sprite. The URL suffix and the Accept header are
// what's available without a webRequest listener.
const ASSET_URL = /\.(png|jpe?g|gif|webp|svg|ico|bmp|avif|css|woff2?|ttf|otf|eot|mp4|webm|mp3|wav|ogg|pdf)([?#]|$)/i;

function looksLikeAsset(msg) {
  if (ASSET_URL.test(msg.url || '')) return true;
  const headers = msg.requestHeaders || {};
  const accept = String(headers.Accept || headers.accept || '').toLowerCase();
  return accept.startsWith('image/') || accept.startsWith('font/') || accept.startsWith('text/css');
}

// --- remembered origins ----------------------------------------------------
// "Помнить вкладки" restores capture by origin after a browser restart, because
// tab ids never survive one.
async function originOf(url) {
  try {
    return new URL(url).origin;
  } catch (_) {
    return '';
  }
}

async function rememberOrigin(origin) {
  if (!origin) return;
  const { captureOrigins = [] } = await chrome.storage.local.get('captureOrigins');
  if (!captureOrigins.includes(origin)) {
    captureOrigins.push(origin);
    await chrome.storage.local.set({ captureOrigins });
  }
}

async function forgetOrigin(origin) {
  if (!origin) return;
  const { captureOrigins = [] } = await chrome.storage.local.get('captureOrigins');
  const next = captureOrigins.filter((o) => o !== origin);
  if (next.length !== captureOrigins.length) await chrome.storage.local.set({ captureOrigins: next });
}

// --- tab description for the popup ----------------------------------------
async function describeTab(tab) {
  if (!tab || tab.id == null) return null;
  return {
    tabId: tab.id,
    title: tab.title || '',
    url: tab.url || '',
    favIconUrl: tab.favIconUrl || '',
    count: capturedCounts.get(tab.id) || 0,
    lastAt: capturedLastAt.get(tab.id) || 0,
    capturing: captureTabIds.has(tab.id),
  };
}

// Hands `json-inspector://open[?tab=N]` to the OS. The extra tab Chrome opens
// for an external protocol is closed right after — otherwise the user is left
// with a blank tab per click.
async function openApp(tabId) {
  const url = 'json-inspector://open' + (tabId != null ? `?tab=${encodeURIComponent(tabId)}` : '');
  try {
    const created = await chrome.tabs.create({ url, active: false });
    setTimeout(() => {
      if (created && created.id != null) chrome.tabs.remove(created.id).catch(() => {});
    }, 1200);
  } catch (_) {
    // Nothing else to try — the popup's own anchor still works.
  }
}

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (!msg) return false;

  // Fire-and-forget from the content bridge — no response expected.
  if (msg.type === 'captured') {
    if (!sender.tab || !captureTabIds.has(sender.tab.id)) return false;
    if (settings.xhrOnly && looksLikeAsset(msg)) return false;
    const id = sender.tab.id;
    capturedCounts.set(id, (capturedCounts.get(id) || 0) + 1);
    capturedLastAt.set(id, Date.now());
    if (connected) setCaptureTitle(id, captureTitleText(id));
    enqueue({
      type: 'request',
      id: makeId(),
      method: msg.method,
      url: msg.url,
      requestHeaders: msg.requestHeaders || {},
      // "Чистить историю при выходе" means the bodies never leave the page —
      // dropping them here also keeps them out of the reconnect buffer.
      requestBody: settings.clearOnExit ? '' : msg.requestBody || '',
      status: msg.status,
      statusText: msg.statusText || '',
      responseHeaders: msg.responseHeaders || {},
      responseBody: settings.clearOnExit ? '' : msg.responseBody || '',
      durationMs: msg.durationMs || 0,
      startedAt: Date.now(),
      tabId: id,
      tabTitle: msg.tabTitle,
      tabURL: msg.tabURL,
      favIconUrl: sender.tab.favIconUrl || '',
    });
    return false;
  }

  // Popup control messages — respond asynchronously.
  (async () => {
    switch (msg.type) {
      case 'getState': {
        const { port } = await readState();
        // An open socket is first-hand evidence the app is up, and it doesn't
        // depend on the health probe being reachable from the worker — a probe
        // that fails while the socket is connected would have the popup tell the
        // user to launch an app they're already talking to.
        const appRunning = connected || (await isAppRunning(port));
        const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
        const activeTabId = tab ? tab.id : null;

        const capturedTabs = [];
        for (const id of captureTabIds) {
          try {
            const t = await chrome.tabs.get(id);
            capturedTabs.push({
              tabId: id,
              title: t.title || '',
              url: t.url || '',
              favIconUrl: t.favIconUrl || '',
              count: capturedCounts.get(id) || 0,
            });
          } catch (_) {
            // Tab closed; onRemoved will clean it up.
          }
        }

        sendResponse({
          capturing: activeTabId != null && captureTabIds.has(activeTabId),
          activeTabId,
          port,
          appRunning,
          capturedTabs,
          currentTab: await describeTab(tab),
          // Requests caught while the app was away; they go out on reconnect.
          bufferedCount: pending.length,
          settings: { ...settings },
        });
        break;
      }
      case 'setSetting': {
        const key = String(msg.key || '');
        if (key in DEFAULT_SETTINGS) {
          settings = { ...settings, [key]: Boolean(msg.value) };
          await chrome.storage.local.set({ settings });
          if (key === 'rememberTabs' && !settings.rememberTabs) {
            await chrome.storage.local.set({ captureOrigins: [] });
          }
        }
        sendResponse({ ok: true, settings: { ...settings } });
        break;
      }
      case 'openApp':
        await openApp(msg.tabId);
        sendResponse({ ok: true });
        break;
      case 'startCapture':
        sendResponse(await startCapture());
        break;
      case 'stopCapture':
        sendResponse(await stopCapture());
        break;
      case 'stopCaptureTab':
        if (msg.tabId != null && captureTabIds.has(msg.tabId)) {
          await stopCaptureFor(msg.tabId);
        }
        sendResponse({ ok: true });
        break;
      case 'setPort': {
        const p = parseInt(msg.port, 10);
        if (p > 0 && p < 65536) {
          await chrome.storage.local.set({ port: p });
          if (captureTabIds.size > 0) connect(p);
        }
        sendResponse({ ok: true });
        break;
      }
    }
  })();
  return true; // keep the message channel open for async sendResponse
});

chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name !== KEEPALIVE) return;
  readState().then(({ port }) => {
    // Nothing to keep alive only when there is neither a capture nor a pause
    // waiting to be resumed.
    if (captureTabIds.size === 0 && !paused) return;
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      connect(port);
    } else {
      socket.send(JSON.stringify({ type: 'ping' }));
    }
  });
});

// Re-inject the interceptor after navigation so capture survives a page reload.
// The per-tab icon and tooltip are reset by Chrome on navigation, so re-apply
// them too.
chrome.webNavigation.onCommitted.addListener((details) => {
  if (details.frameId !== 0) return;
  if (!captureTabIds.has(details.tabId)) return;
  refreshIndicator();
  injectInto(details.tabId).catch(() => {});
  chrome.tabs.get(details.tabId).then((t) => {
    if (captureTabIds.has(details.tabId)) {
      captureTabMeta.set(details.tabId, { title: t.title || '', url: t.url || '' });
    }
  }).catch(() => {});
});

// "Помнить вкладки": tab ids are gone after a restart, so capture is restored
// by origin instead — every tab on a remembered origin starts recording again.
chrome.runtime.onStartup.addListener(async () => {
  await readState();
  if (!settings.rememberTabs) return;
  const { captureOrigins = [] } = await chrome.storage.local.get('captureOrigins');
  await captureByOrigins(captureOrigins);
});

// A paused extension has to come back paused: the app is still showing
// "Возобновить перехват", and that command needs this side connected to land.
chrome.storage.session.get(['captureTabIds', 'pausedOrigins']).then(({ pausedOrigins: pausedList, captureTabIds: ids }) => {
  if (pausedList && pausedList.length) {
    paused = true;
    pausedOrigins = pausedList;
    chrome.alarms.create(KEEPALIVE, { periodInMinutes: 0.4 });
    readState().then(({ port }) => connect(port));
    return;
  }
  restoreCaptured(ids || []);
});

// Restore capture state if the service worker was killed and restarted.
function restoreCaptured(ids) {
  if (!ids || !ids.length) return;
  for (const id of ids) {
    captureTabIds.add(id);
    captureTabMeta.set(id, { title: '', url: '' });
    setCaptureIcon(id, 'disconnected');
    setCaptureTitle(id, 'Перехват: приложение недоступно');
  }
  readState().then(({ port }) => connect(port));
}

// If a captured tab closes, remove it from the capture set.
chrome.tabs.onRemoved.addListener((tabId) => {
  if (!captureTabIds.has(tabId)) return;
  captureTabIds.delete(tabId);
  captureTabMeta.delete(tabId);
  capturedCounts.delete(tabId);
  capturedLastAt.delete(tabId);
  persistCapture();
  if (captureTabIds.size === 0) {
    chrome.alarms.clear(KEEPALIVE);
    disconnect();
  }
  sendState();
});
