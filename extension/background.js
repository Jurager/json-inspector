const DEFAULT_PORT = 38761;
const KEEPALIVE = 'ji-keepalive';
const RECONNECT_MS = 2500;

const RECORDING_ICON = {
  16: 'icons/icon16.png',
  32: 'icons/icon32.png',
  48: 'icons/icon48.png',
  128: 'icons/icon128.png',
};

const OFF_ICON = {
  16: 'icons/off-icon16.png',
  32: 'icons/off-icon32.png',
  48: 'icons/off-icon48.png',
  128: 'icons/off-icon128.png',
};

const DISCONNECTED_ICON = {
  16: 'icons/disconnected-icon16.png',
  32: 'icons/disconnected-icon32.png',
  48: 'icons/disconnected-icon48.png',
  128: 'icons/disconnected-icon128.png',
};

const DEFAULT_SETTINGS = {
  rememberTabs: true,
  xhrOnly: true,
  clearOnExit: false,
};

const ASSET_URL =
    /\.(png|jpe?g|gif|webp|svg|ico|bmp|avif|css|woff2?|ttf|otf|eot|mp4|webm|mp3|wav|ogg|pdf)([?#]|$)/i;

let socket = null;
let reconnectTimer = null;
let pending = [];
let captureTabIds = new Set();
let captureTabMeta = new Map();
let capturedCounts = new Map();
let capturedLastAt = new Map();
let settings = { ...DEFAULT_SETTINGS };
let connected = false;
let pausedOrigins = [];
let paused = false;

chrome.storage.local.get('settings').then(({ settings: stored }) => {
  settings = {
    ...DEFAULT_SETTINGS,
    ...(stored || {}),
  };
});

chrome.storage.onChanged.addListener((changes, area) => {
  if (area !== 'local' || !changes.settings) {
    return;
  }

  settings = {
    ...DEFAULT_SETTINGS,
    ...(changes.settings.newValue || {}),
  };
});

async function readState() {
  const [session, local] = await Promise.all([
    chrome.storage.session.get('captureTabIds'),
    chrome.storage.local.get({
      port: DEFAULT_PORT,
      settings: DEFAULT_SETTINGS,
    }),
  ]);

  settings = {
    ...DEFAULT_SETTINGS,
    ...(local.settings || {}),
  };

  return {
    captureTabIds: session.captureTabIds ?? [],
    port: local.port || DEFAULT_PORT,
  };
}

async function persistCapture() {
  await chrome.storage.session.set({
    captureTabIds: [...captureTabIds],
  });
}

function setCaptureIcon(tabId, state) {
  const path = {
    recording: RECORDING_ICON,
    disconnected: DISCONNECTED_ICON,
    normal: OFF_ICON,
  }[state];

  chrome.action.setIcon({
    path,
    tabId,
  });
}

function setCaptureTitle(tabId, title) {
  chrome.action.setTitle({
    title,
    tabId,
  });
}

function pluralRequests(count) {
  const mod10 = count % 10;
  const mod100 = count % 100;

  if (mod10 === 1 && mod100 !== 11) {
    return `${count} запрос`;
  }

  if (
      mod10 >= 2 &&
      mod10 <= 4 &&
      (mod100 < 12 || mod100 > 14)
  ) {
    return `${count} запроса`;
  }

  return `${count} запросов`;
}

function captureTitleText(tabId) {
  const meta = captureTabMeta.get(tabId);
  const label = meta?.title ? `«${meta.title}»` : 'вкладка';
  const count = capturedCounts.get(tabId) || 0;

  return `Перехват: ${label} · ${pluralRequests(count)}`;
}

function refreshIndicator() {
  for (const tabId of captureTabIds) {
    setCaptureIcon(
        tabId,
        connected ? 'recording' : 'disconnected'
    );

    setCaptureTitle(
        tabId,
        connected
            ? captureTitleText(tabId)
            : 'Перехват: приложение недоступно'
    );
  }
}

async function isAppRunning(port) {
  try {
    const response = await fetch(
        `http://127.0.0.1:${port}/health`,
        {
          signal: AbortSignal.timeout(2000),
        }
    );

    return response.ok;
  } catch {
    return false;
  }
}

async function connect(port) {
  clearTimeout(reconnectTimer);

  if (socket?.readyState === WebSocket.OPEN) {
    connected = true;
    return;
  }

  if (socket?.readyState === WebSocket.CONNECTING) {
    return;
  }

  if (socket) {
    try {
      socket.close();
    } catch {}

    socket = null;
  }

  connected = false;

  if (!(await isAppRunning(port))) {
    refreshIndicator();
    scheduleReconnect();
    return;
  }

  let ws;

  try {
    ws = new WebSocket(`ws://127.0.0.1:${port}`);
  } catch {
    scheduleReconnect();
    return;
  }

  socket = ws;

  ws.onopen = () => {
    if (socket !== ws) {
      return;
    }

    connected = true;
    refreshIndicator();
    sendState();

    for (const message of pending.splice(0)) {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(message);
      }
    }
  };

  ws.onclose = () => {
    if (socket !== ws) {
      return;
    }

    socket = null;
    connected = false;
    refreshIndicator();
    scheduleReconnect();
  };

  ws.onerror = () => {};

  ws.onmessage = ({ data }) => {
    if (socket !== ws) {
      return;
    }

    let message;

    try {
      message = JSON.parse(data);
    } catch {
      return;
    }

    if (message.type === 'pause') {
      pauseAll();
    }

    if (message.type === 'resume') {
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

    if (captureTabIds.size > 0 || paused) {
      connect(port);
    }
  }, RECONNECT_MS);
}

function disconnect() {
  clearTimeout(reconnectTimer);

  if (socket) {
    try {
      socket.close();
    } catch {}

    socket = null;
  }

  connected = false;
  pending = [];
}

function enqueue(payload) {
  const message = JSON.stringify(payload);

  if (socket?.readyState === WebSocket.OPEN) {
    socket.send(message);
    return;
  }

  pending.push(message);

  if (pending.length > 200) {
    pending.shift();
  }
}

function sendState() {
  if (socket?.readyState !== WebSocket.OPEN) {
    return;
  }

  socket.send(
      JSON.stringify({
        type: 'state',
        recording: connected && captureTabIds.size > 0,
        tabs: captureTabIds.size,
        browser: 'Chrome',
      })
  );
}

const BLOCKED_SCHEMES = [
  'chrome:',
  'chrome-untrusted:',
  'chrome-extension:',
  'edge:',
  'devtools:',
  'about:',
  'view-source:',
];

function isBlockedPage(url) {
  let parsed = null;

  try {
    parsed = new URL(url || '');
  } catch {
    return false;
  }

  return (
      BLOCKED_SCHEMES.includes(parsed.protocol) ||
      parsed.hostname === 'chromewebstore.google.com' ||
      (parsed.hostname === 'chrome.google.com' &&
          parsed.pathname.startsWith('/webstore'))
  );
}

function injectionFailure(tab, err) {
  console.warn(
      '[json-inspector] injection into',
      tab?.url || '(unknown)',
      'failed:',
      err
  );

  return isBlockedPage(tab?.url)
      ? 'Chrome не разрешает перехват на этой странице'
      : 'Не удалось подключиться к странице';
}

async function injectInto(tabId) {
  await chrome.scripting.executeScript({
    target: { tabId },
    files: ['content/bridge.js'],
  });

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
      func: () => {
        window.__jsonInspectorEnabled = false;
      },
      world: 'MAIN',
    });
  } catch {}
}

function addCaptureTab(tab, resetCount = true) {
  if (tab?.id == null) {
    return;
  }

  captureTabIds.add(tab.id);

  captureTabMeta.set(tab.id, {
    title: tab.title || '',
    url: tab.url || '',
  });

  if (resetCount) {
    capturedCounts.set(tab.id, 0);
  }
}

async function startCapture() {
  const [tab] = await chrome.tabs.query({
    active: true,
    currentWindow: true,
  });

  if (tab?.id == null) {
    return {
      ok: false,
      error: 'нет активной вкладки',
    };
  }

  if (!captureTabIds.has(tab.id)) {
    try {
      await injectInto(tab.id);
    } catch (err) {
      return {
        ok: false,
        error: injectionFailure(tab, err),
      };
    }

    paused = false;
    addCaptureTab(tab);

    if (settings.rememberTabs) {
      await rememberOrigin(originOf(tab.url));
    }

    await persistCapture();
  }

  chrome.alarms.create(KEEPALIVE, {
    periodInMinutes: 0.4,
  });

  setCaptureIcon(tab.id, 'disconnected');
  setCaptureTitle(
      tab.id,
      'Перехват: приложение недоступно'
  );

  if (socket?.readyState !== WebSocket.OPEN) {
    const { port } = await readState();
    connect(port);
  } else {
    refreshIndicator();
    sendState();
  }

  return {
    ok: true,
    tabId: tab.id,
  };
}

async function stopCaptureFor(tabId, keepConnection = false) {
  await disableInTab(tabId);

  setCaptureIcon(tabId, 'normal');
  setCaptureTitle(tabId, 'JSON Inspector');

  const meta = captureTabMeta.get(tabId);

  captureTabIds.delete(tabId);
  captureTabMeta.delete(tabId);
  capturedCounts.delete(tabId);
  capturedLastAt.delete(tabId);

  if (!keepConnection && meta?.url) {
    const origin = originOf(meta.url);

    const stillUsed = await Promise.all(
        [...captureTabIds].map(async (id) => {
          try {
            const tab = await chrome.tabs.get(id);
            return originOf(tab.url) === origin;
          } catch {
            return false;
          }
        })
    );

    if (!stillUsed.some(Boolean)) {
      await forgetOrigin(origin);
    }
  }

  await persistCapture();

  if (captureTabIds.size === 0 && !keepConnection) {
    chrome.alarms.clear(KEEPALIVE);
    disconnect();
  }

  sendState();
}

async function stopCapture() {
  const [tab] = await chrome.tabs.query({
    active: true,
    currentWindow: true,
  });

  if (tab?.id == null) {
    return { ok: true };
  }

  await stopCaptureFor(tab.id);

  return { ok: true };
}

async function captureByOrigins(origins) {
  if (!origins.length) {
    return 0;
  }

  const tabs = await chrome.tabs.query({});

  for (const tab of tabs) {
    if (
        tab.id == null ||
        !tab.url ||
        captureTabIds.has(tab.id) ||
        !origins.includes(originOf(tab.url))
    ) {
      continue;
    }

    try {
      await injectInto(tab.id);
    } catch {
      continue;
    }

    addCaptureTab(tab);
  }

  if (captureTabIds.size === 0) {
    return 0;
  }

  await persistCapture();

  chrome.alarms.create(KEEPALIVE, {
    periodInMinutes: 0.4,
  });

  refreshIndicator();

  const { port } = await readState();

  await connect(port);
  sendState();

  return captureTabIds.size;
}

async function pauseAll() {
  const origins = [
    ...new Set(
        [...captureTabIds]
            .map((tabId) => captureTabMeta.get(tabId)?.url)
            .map(originOf)
            .filter(Boolean)
    ),
  ];

  pausedOrigins = origins;
  paused = true;

  await chrome.storage.session.set({
    pausedOrigins: origins,
  });

  for (const tabId of [...captureTabIds]) {
    await stopCaptureFor(tabId, true);
  }

  chrome.alarms.create(KEEPALIVE, {
    periodInMinutes: 0.4,
  });

  const { port } = await readState();

  if (
      socket?.readyState !== WebSocket.OPEN &&
      socket?.readyState !== WebSocket.CONNECTING
  ) {
    connect(port);
  }

  sendState();
}

async function resumeAll() {
  const [session, remembered] = await Promise.all([
    chrome.storage.session.get('pausedOrigins'),
    chrome.storage.local.get('captureOrigins'),
  ]);

  const origins =
      pausedOrigins.length
          ? pausedOrigins
          : session.pausedOrigins?.length
              ? session.pausedOrigins
              : remembered.captureOrigins || [];

  pausedOrigins = [];
  paused = false;

  await chrome.storage.session.set({
    pausedOrigins: [],
  });

  await captureByOrigins(origins);
}

function makeId() {
  return (
      `${Date.now().toString(36)}-` +
      Math.random().toString(36).slice(2, 8)
  );
}

function looksLikeAsset(message) {
  if (ASSET_URL.test(message.url || '')) {
    return true;
  }

  const headers = message.requestHeaders || {};
  const accept = String(
      headers.Accept || headers.accept || ''
  ).toLowerCase();

  return (
      accept.startsWith('image/') ||
      accept.startsWith('font/') ||
      accept.startsWith('text/css')
  );
}

function originOf(url) {
  try {
    return new URL(url).origin;
  } catch {
    return '';
  }
}

async function rememberOrigin(origin) {
  if (!origin) {
    return;
  }

  const { captureOrigins = [] } =
      await chrome.storage.local.get('captureOrigins');

  if (captureOrigins.includes(origin)) {
    return;
  }

  captureOrigins.push(origin);

  await chrome.storage.local.set({
    captureOrigins,
  });
}

async function forgetOrigin(origin) {
  if (!origin) {
    return;
  }

  const { captureOrigins = [] } =
      await chrome.storage.local.get('captureOrigins');

  const next = captureOrigins.filter(
      (item) => item !== origin
  );

  if (next.length !== captureOrigins.length) {
    await chrome.storage.local.set({
      captureOrigins: next,
    });
  }
}

async function describeTab(tab) {
  if (tab?.id == null) {
    return null;
  }

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

async function getCapturedTabs() {
  const tabs = [];

  for (const tabId of captureTabIds) {
    try {
      const tab = await chrome.tabs.get(tabId);

      tabs.push({
        tabId,
        title: tab.title || '',
        url: tab.url || '',
        favIconUrl: tab.favIconUrl || '',
        count: capturedCounts.get(tabId) || 0,
      });
    } catch {}
  }

  return tabs;
}

chrome.runtime.onMessage.addListener(
    (message, sender, sendResponse) => {
      if (!message) {
        return false;
      }

      if (message.type === 'captured') {
        handleCapturedRequest(message, sender);
        return false;
      }

      handleRuntimeMessage(message, sendResponse);

      return true;
    }
);

function handleCapturedRequest(message, sender) {
  if (
      !sender.tab ||
      !captureTabIds.has(sender.tab.id)
  ) {
    return;
  }

  if (settings.xhrOnly && looksLikeAsset(message)) {
    return;
  }

  const tabId = sender.tab.id;

  capturedCounts.set(
      tabId,
      (capturedCounts.get(tabId) || 0) + 1
  );

  capturedLastAt.set(tabId, Date.now());

  if (connected) {
    setCaptureTitle(
        tabId,
        captureTitleText(tabId)
    );
  }

  enqueue({
    type: 'request',
    id: makeId(),
    method: message.method,
    url: message.url,
    requestHeaders: message.requestHeaders || {},
    requestBody: settings.clearOnExit
        ? ''
        : message.requestBody || '',
    status: message.status,
    statusText: message.statusText || '',
    responseHeaders: message.responseHeaders || {},
    responseBody: settings.clearOnExit
        ? ''
        : message.responseBody || '',
    durationMs: message.durationMs || 0,
    hasTiming: Boolean(message.hasTiming),
    waitMs: message.waitMs || 0,
    downloadMs: message.downloadMs || 0,
    startedAt: Date.now(),
    tabId,
    tabTitle: message.tabTitle,
    tabURL: message.tabURL,
    favIconUrl: sender.tab.favIconUrl || '',
  });
}

async function handleRuntimeMessage(message, sendResponse) {
  try {
    await respond(message, sendResponse);
  } catch (err) {
    console.warn(
        '[json-inspector] message', message?.type, 'failed:', err
    );

    sendResponse({
      ok: false,
      error: 'Сбой в расширении: ' + String(err?.message || err || 'неизвестная ошибка'),
    });
  }
}

async function respond(message, sendResponse) {
  switch (message.type) {
    case 'getState':
      sendResponse(await getState());
      break;

    case 'focusApp':
      sendResponse(focusApp(message.tabId));
      break;

    case 'setSetting':
      sendResponse(await setSetting(message));
      break;

    case 'startCapture':
      sendResponse(await startCapture());
      break;

    case 'stopCapture':
      sendResponse(await stopCapture());
      break;

    case 'stopCaptureTab':
      if (
          message.tabId != null &&
          captureTabIds.has(message.tabId)
      ) {
        await stopCaptureFor(message.tabId);
      }

      sendResponse({ ok: true });
      break;

    case 'setPort':
      sendResponse(await setPort(message.port));
      break;

    default:
      sendResponse({
        ok: false,
        error: 'неизвестная команда: ' + String(message.type),
      });
      break;
  }
}

async function getState() {
  const { port } = await readState();

  const appRunning =
      connected || (await isAppRunning(port));

  const [tab] = await chrome.tabs.query({
    active: true,
    currentWindow: true,
  });

  return {
    capturing:
        tab?.id != null &&
        captureTabIds.has(tab.id),
    activeTabId: tab?.id ?? null,
    port,
    appRunning,
    capturedTabs: await getCapturedTabs(),
    currentTab: await describeTab(tab),
    bufferedCount: pending.length,
    settings: { ...settings },
  };
}

function focusApp(tabId) {
  const live =
      socket?.readyState === WebSocket.OPEN;

  if (live) {
    socket.send(
        JSON.stringify({
          type: 'focus',
          tab: tabId ?? 0,
        })
    );
  }

  return {
    ok: Boolean(live),
  };
}

async function setSetting(message) {
  const key = String(message.key || '');

  if (key in DEFAULT_SETTINGS) {
    settings = {
      ...settings,
      [key]: Boolean(message.value),
    };

    await chrome.storage.local.set({ settings });

    if (
        key === 'rememberTabs' &&
        !settings.rememberTabs
    ) {
      await chrome.storage.local.set({
        captureOrigins: [],
      });
    }
  }

  return {
    ok: true,
    settings: { ...settings },
  };
}

async function setPort(value) {
  const port = parseInt(value, 10);

  if (port > 0 && port < 65536) {
    await chrome.storage.local.set({ port });

    if (captureTabIds.size > 0 || paused) {
      disconnect();
      connect(port);
    }
  }

  return { ok: true };
}

chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name !== KEEPALIVE) {
    return;
  }

  readState().then(({ port }) => {
    if (captureTabIds.size === 0 && !paused) {
      return;
    }

    if (socket?.readyState !== WebSocket.OPEN) {
      connect(port);
      return;
    }

    socket.send(
        JSON.stringify({
          type: 'ping',
        })
    );
  });
});

chrome.webNavigation.onCommitted.addListener(
    (details) => {
      if (
          details.frameId !== 0 ||
          !captureTabIds.has(details.tabId)
      ) {
        return;
      }

      refreshIndicator();

      injectInto(details.tabId).catch(() => {});

      chrome.tabs
          .get(details.tabId)
          .then((tab) => {
            if (!captureTabIds.has(details.tabId)) {
              return;
            }

            captureTabMeta.set(details.tabId, {
              title: tab.title || '',
              url: tab.url || '',
            });
          })
          .catch(() => {});
    }
);

chrome.runtime.onStartup.addListener(async () => {
  await readState();

  if (!settings.rememberTabs) {
    return;
  }

  const { captureOrigins = [] } =
      await chrome.storage.local.get('captureOrigins');

  await captureByOrigins(captureOrigins);
});

chrome.storage.session
    .get(['captureTabIds', 'pausedOrigins'])
    .then(
        ({
           pausedOrigins: savedPausedOrigins,
           captureTabIds,
         }) => {
          if (savedPausedOrigins?.length) {
            paused = true;
            pausedOrigins = savedPausedOrigins;

            chrome.alarms.create(KEEPALIVE, {
              periodInMinutes: 0.4,
            });

            readState().then(({ port }) => {
              connect(port);
            });

            return;
          }

          restoreCaptured(captureTabIds || []);
        }
    );

function restoreCaptured(tabIds) {
  if (!tabIds.length) {
    return;
  }

  for (const tabId of tabIds) {
    captureTabIds.add(tabId);

    captureTabMeta.set(tabId, {
      title: '',
      url: '',
    });

    setCaptureIcon(tabId, 'disconnected');
    setCaptureTitle(
        tabId,
        'Перехват: приложение недоступно'
    );
  }

  readState().then(({ port }) => {
    connect(port);
  });
}

chrome.tabs.onRemoved.addListener((tabId) => {
  if (!captureTabIds.has(tabId)) {
    return;
  }

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