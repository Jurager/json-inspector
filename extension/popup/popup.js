const DEFAULT_PORT = 38761;

async function getState() {
  return await chrome.runtime.sendMessage({ type: 'getState' });
}

function hostnameOf(url) {
  try {
    return new URL(url).hostname;
  } catch (_) {
    return '';
  }
}

function formatCount(n) {
  const m10 = n % 10;
  const m100 = n % 100;
  if (m10 === 1 && m100 !== 11) return n + ' запрос';
  if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return n + ' запроса';
  return n + ' запросов';
}

function render(state) {
  const appDot = document.getElementById('app-dot');
  const appStatus = document.getElementById('app-status');
  if (state.appRunning) {
    appDot.className = 'dot ok';
    appStatus.textContent = 'Приложение запущено';
  } else {
    appDot.className = 'dot warn';
    appStatus.textContent = 'Приложение не запущено';
  }

  document.getElementById('enabled').checked = state.capturing;
  renderTabs(state.capturedTabs || []);

  const btn = document.getElementById('open-app');
  if (state.appRunning) {
    btn.className = 'open-app secondary';
    btn.textContent = 'Показать приложение';
  } else {
    btn.className = 'open-app primary';
    btn.textContent = 'Открыть приложение';
  }
}

function renderTabs(tabs) {
  const section = document.getElementById('tabs-section');
  const list = document.getElementById('tabs-list');
  list.textContent = '';
  if (tabs.length === 0) {
    section.style.display = 'none';
    return;
  }
  section.style.display = '';
  for (const t of tabs) list.appendChild(buildTabItem(t));
}

function buildTabItem(t) {
  const item = document.createElement('div');
  item.className = 'tab-item';

  if (t.favIconUrl) {
    const img = document.createElement('img');
    img.className = 'favicon';
    img.src = t.favIconUrl;
    img.alt = '';
    img.addEventListener('error', () => img.classList.add('hidden'));
    item.appendChild(img);
  }

  const host = hostnameOf(t.url);

  const text = document.createElement('div');
  text.className = 'tab-item-text';

  const title = document.createElement('div');
  title.className = 'tab-item-title';
  title.textContent = t.title || host || 'вкладка';
  text.appendChild(title);

  if (host && t.title && host !== t.title) {
    const h = document.createElement('div');
    h.className = 'tab-item-host';
    h.textContent = host;
    text.appendChild(h);
  }
  item.appendChild(text);

  const count = document.createElement('span');
  count.className = 'tab-item-count';
  count.textContent = formatCount(t.count || 0);
  item.appendChild(count);

  const stop = document.createElement('button');
  stop.className = 'tab-item-stop';
  stop.title = 'Остановить перехват этой вкладки';
  stop.textContent = '×';
  stop.addEventListener('click', async () => {
    await chrome.runtime.sendMessage({ type: 'stopCaptureTab', tabId: t.tabId });
    const state = await getState();
    render(state);
  });
  item.appendChild(stop);

  return item;
}

async function init() {
  const state = await getState();
  document.getElementById('port').value = state.port || DEFAULT_PORT;
  render(state);
}

let pollTimer = null;
function startPolling() {
  stopPolling();
  pollTimer = setInterval(async () => {
    try {
      const state = await getState();
      render(state);
    } catch (_) {}
  }, 2500);
}
function stopPolling() {
  if (pollTimer) clearInterval(pollTimer);
  pollTimer = null;
}

document.getElementById('enabled').addEventListener('change', async (e) => {
  if (e.target.checked) {
    await chrome.runtime.sendMessage({ type: 'startCapture' });
  } else {
    await chrome.runtime.sendMessage({ type: 'stopCapture' });
  }
  const state = await getState();
  render(state);
});

document.getElementById('port').addEventListener('change', async (e) => {
  const p = parseInt(e.target.value, 10);
  if (p > 0 && p < 65536) await chrome.runtime.sendMessage({ type: 'setPort', port: p });
});

init();
startPolling();
window.addEventListener('unload', stopPolling);
