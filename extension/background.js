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

let socket = null;
let reconnectTimer = null;
let pending = [];
let captureTabIds = new Set(); // tabs currently being captured
let captureTabMeta = new Map(); // tabId -> { title }
let capturedCounts = new Map(); // tabId -> number of captured requests
let connected = false;

async function readState() {
  const sess = await chrome.storage.session.get('captureTabIds');
  const local = await chrome.storage.local.get('port');
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
}

function scheduleReconnect() {
  clearTimeout(reconnectTimer);
  connected = false;
  refreshIndicator();
  reconnectTimer = setTimeout(async () => {
    const { port } = await readState();
    if (captureTabIds.size > 0) connect(port);
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

  if (!captureTabIds.has(tab.id)) {
    await injectInto(tab.id);
    captureTabIds.add(tab.id);
    captureTabMeta.set(tab.id, { title: tab.title || '' });
    capturedCounts.set(tab.id, 0);
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

async function stopCaptureFor(tabId) {
  await disableInTab(tabId);
  setCaptureIcon(tabId, 'normal');
  setCaptureTitle(tabId, 'JSON Inspector');
  captureTabIds.delete(tabId);
  captureTabMeta.delete(tabId);
  capturedCounts.delete(tabId);
  await persistCapture();
  if (captureTabIds.size === 0) {
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

function makeId() {
  return Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 8);
}

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (!msg) return false;

  // Fire-and-forget from the content bridge — no response expected.
  if (msg.type === 'captured') {
    if (!sender.tab || !captureTabIds.has(sender.tab.id)) return false;
    const id = sender.tab.id;
    capturedCounts.set(id, (capturedCounts.get(id) || 0) + 1);
    if (connected) setCaptureTitle(id, captureTitleText(id));
    enqueue({
      type: 'request',
      id: makeId(),
      method: msg.method,
      url: msg.url,
      requestHeaders: msg.requestHeaders || {},
      requestBody: msg.requestBody || '',
      status: msg.status,
      statusText: msg.statusText || '',
      responseHeaders: msg.responseHeaders || {},
      responseBody: msg.responseBody || '',
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
        const appRunning = await isAppRunning(port);
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
        });
        break;
      }
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
    if (captureTabIds.size === 0) return;
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
    if (captureTabIds.has(details.tabId)) captureTabMeta.set(details.tabId, { title: t.title || '' });
  }).catch(() => {});
});

// Restore capture state if the service worker was killed and restarted.
chrome.storage.session.get('captureTabIds').then(({ captureTabIds: ids }) => {
  if (!ids || !ids.length) return;
  for (const id of ids) {
    captureTabIds.add(id);
    captureTabMeta.set(id, { title: '' });
    setCaptureIcon(id, 'disconnected');
    setCaptureTitle(id, 'Перехват: приложение недоступно');
  }
  readState().then(({ port }) => connect(port));
});

// If a captured tab closes, remove it from the capture set.
chrome.tabs.onRemoved.addListener((tabId) => {
  if (!captureTabIds.has(tabId)) return;
  captureTabIds.delete(tabId);
  captureTabMeta.delete(tabId);
  capturedCounts.delete(tabId);
  persistCapture();
  if (captureTabIds.size === 0) {
    chrome.alarms.clear(KEEPALIVE);
    disconnect();
  }
  sendState();
});
