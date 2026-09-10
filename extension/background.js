// Service worker. Captures requests from ONE tab at a time (chosen via the
// popup) and forwards them to the JSON Inspector app over a WebSocket. Capture
// state is shown by swapping the toolbar icon: a red dot while the app is
// connected, an orange dot while the app is unreachable. A silent /health check
// runs before each connection attempt so a closed app never logs WebSocket
// errors, and the interceptor is re-injected on navigation.
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
let captureTabId = null;
let connected = false;

async function getState() {
  const sess = await chrome.storage.session.get('captureTabId');
  const local = await chrome.storage.local.get('port');
  return { captureTabId: sess.captureTabId ?? null, port: local.port || DEFAULT_PORT };
}

function setCaptureIcon(tabId, state) {
  const path = state === 'recording' ? RECORDING_ICON : state === 'disconnected' ? DISCONNECTED_ICON : NORMAL_ICON;
  chrome.action.setIcon({ path, tabId });
}

function refreshIcon() {
  if (captureTabId == null) return;
  setCaptureIcon(captureTabId, connected ? 'recording' : 'disconnected');
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
    refreshIcon();
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
    refreshIcon();
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
  refreshIcon();
  reconnectTimer = setTimeout(async () => {
    const { captureTabId: id, port } = await getState();
    if (id != null) connect(port);
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

  const prev = await getState();
  if (prev.captureTabId != null && prev.captureTabId !== tab.id) {
    await disableInTab(prev.captureTabId);
    setCaptureIcon(prev.captureTabId, 'normal');
  }

  await injectInto(tab.id);

  captureTabId = tab.id;
  await chrome.storage.session.set({ captureTabId: tab.id });
  chrome.alarms.create(KEEPALIVE, { periodInMinutes: 0.4 });
  setCaptureIcon(tab.id, 'disconnected');

  const { port } = await getState();
  connect(port);
  return { ok: true, tabId: tab.id };
}

async function stopCapture() {
  const { captureTabId: id } = await getState();
  if (id != null) {
    await disableInTab(id);
    setCaptureIcon(id, 'normal');
  }
  captureTabId = null;
  await chrome.storage.session.remove('captureTabId');
  chrome.alarms.clear(KEEPALIVE);
  disconnect();
  return { ok: true };
}

function makeId() {
  return Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 8);
}

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (!msg) return false;

  // Fire-and-forget from the content bridge — no response expected.
  if (msg.type === 'captured') {
    chrome.storage.session.get('captureTabId').then(({ captureTabId: id }) => {
      if (id == null || !sender.tab || sender.tab.id !== id) return;
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
        tabId: sender.tab.id,
        tabTitle: msg.tabTitle,
        tabURL: msg.tabURL,
      });
    });
    return false;
  }

  // Popup control messages — respond asynchronously.
  (async () => {
    switch (msg.type) {
      case 'getState': {
        const { captureTabId: id, port } = await getState();
        const appRunning = await isAppRunning(port);
        const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
        sendResponse({
          capturing: id != null,
          captureTabId: id,
          activeTabId: tab ? tab.id : null,
          port,
          appRunning,
        });
        break;
      }
      case 'startCapture':
        sendResponse(await startCapture());
        break;
      case 'stopCapture':
        sendResponse(await stopCapture());
        break;
      case 'setPort': {
        const p = parseInt(msg.port, 10);
        if (p > 0 && p < 65536) {
          await chrome.storage.local.set({ port: p });
          const { captureTabId: id } = await getState();
          if (id != null) connect(p);
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
  getState().then(({ captureTabId: id, port }) => {
    if (id == null) return;
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      connect(port);
    } else {
      socket.send(JSON.stringify({ type: 'ping' }));
    }
  });
});

// Re-inject the interceptor after navigation so capture survives a page reload.
// The per-tab icon is reset by Chrome on navigation, so re-apply it too.
chrome.webNavigation.onCommitted.addListener((details) => {
  if (details.frameId !== 0) return;
  if (details.tabId !== captureTabId) return;
  refreshIcon();
  injectInto(details.tabId).catch(() => {});
});

// Restore capture state if the service worker was killed and restarted.
chrome.storage.session.get('captureTabId').then(({ captureTabId: id }) => {
  if (id != null) {
    captureTabId = id;
    setCaptureIcon(id, 'disconnected');
    getState().then(({ port }) => connect(port));
  }
});

// If the captured tab closes, stop capture.
chrome.tabs.onRemoved.addListener((tabId) => {
  if (tabId === captureTabId) {
    captureTabId = null;
    chrome.storage.session.remove('captureTabId');
    chrome.alarms.clear(KEEPALIVE);
    disconnect();
  }
});
