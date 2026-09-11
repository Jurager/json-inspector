const DEFAULT_PORT = 38761;

async function getState() {
  return await chrome.runtime.sendMessage({ type: 'getState' });
}

function $(id) {
  return document.getElementById(id);
}

function hostnameOf(url) {
  try {
    return new URL(url).hostname;
  } catch (_) {
    return '';
  }
}

function plural(n, one, few, many) {
  const m10 = n % 10;
  const m100 = n % 100;
  if (m10 === 1 && m100 !== 11) return `${n} ${one}`;
  if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return `${n} ${few}`;
  return `${n} ${many}`;
}

const requestsLabel = (n) => plural(n, 'запрос', 'запроса', 'запросов');

// "12 с назад" / "3 мин назад" — enough resolution for "is this thing alive".
function agoLabel(at) {
  if (!at) return '';
  const sec = Math.max(0, Math.round((Date.now() - at) / 1000));
  if (sec < 60) return `последний ${sec} с назад`;
  const min = Math.round(sec / 60);
  if (min < 60) return `последний ${min} мин назад`;
  return `последний ${Math.round(min / 60)} ч назад`;
}

// A tab is identified by its favicon when there is one, and by a lettered tile
// when there isn't (or when the icon fails to load).
function buildIcon(favIconUrl, label) {
  const wrap = document.createElement('span');
  wrap.className = 'icon-slot';
  const initial = (label || '?').trim().charAt(0) || '?';

  const avatar = document.createElement('span');
  avatar.className = 'avatar';
  avatar.textContent = initial;
  // Exactly one of the two is visible: a fresh element is visible by default,
  // so the avatar has to be hidden explicitly whenever an icon is expected.
  avatar.hidden = Boolean(favIconUrl);

  if (favIconUrl) {
    const img = document.createElement('img');
    img.className = 'favicon';
    img.src = favIconUrl;
    img.alt = '';
    // A broken icon falls back to the letter tile rather than an empty box.
    img.addEventListener('error', () => {
      img.classList.add('hidden');
      avatar.hidden = false;
    });
    wrap.appendChild(img);
  }
  wrap.appendChild(avatar);
  return wrap;
}

function buildMoreRow(t) {
  const row = document.createElement('div');
  row.className = 'more-row';

  const host = hostnameOf(t.url);
  row.appendChild(buildIcon(t.favIconUrl, t.title || host));

  const text = document.createElement('div');
  text.className = 'more-text';
  const title = document.createElement('div');
  title.className = 'more-title';
  title.textContent = t.title || host || 'Вкладка';
  const hostEl = document.createElement('div');
  hostEl.className = 'more-host mono';
  hostEl.textContent = host;
  text.append(title, hostEl);
  row.appendChild(text);

  const count = document.createElement('span');
  count.className = 'more-count';
  count.textContent = String(t.count || 0);
  row.appendChild(count);

  const stop = document.createElement('button');
  stop.className = 'more-stop';
  stop.title = 'Остановить перехват этой вкладки';
  stop.innerHTML =
    '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><path d="M6 6l12 12M18 6 6 18"/></svg>';
  stop.addEventListener('click', async (e) => {
    e.stopPropagation();
    await chrome.runtime.sendMessage({ type: 'stopCaptureTab', tabId: t.tabId });
    render(await getState());
  });
  row.appendChild(stop);

  // Anywhere else on the row: take me to this tab's requests.
  row.addEventListener('click', () => chrome.runtime.sendMessage({ type: 'openApp', tabId: t.tabId }));
  return row;
}

function renderHeader(state) {
  const dot = $('status-dot');
  const text = $('status-text');
  const port = $('status-port');
  if (state.appRunning) {
    dot.className = 'dot ok';
    text.textContent = 'Подключено';
    port.textContent = ':' + (state.port || DEFAULT_PORT);
  } else {
    dot.className = 'dot warn';
    text.textContent = 'Приложение не запущено';
    // The port moves into the warning card, where the number is actionable.
    port.textContent = '';
  }
}

function renderCurrentTab(state) {
  const card = $('current-card');
  const tab = state.currentTab;
  const capturing = Boolean(tab && tab.capturing);

  card.className = 'card current ' + (capturing ? 'on' : 'off') + (state.appRunning ? '' : ' muted');

  if (!tab) {
    $('current-title').textContent = 'Нет активной вкладки';
    $('current-host').textContent = '';
    $('enabled').checked = false;
    $('current-count').textContent = '';
    $('current-ago').textContent = '';
    $('current-arrow').hidden = true;
    $('current-foot').classList.remove('actionable');
    return;
  }

  const host = hostnameOf(tab.url);
  const label = tab.title || host || 'Вкладка';

  // Rebuild the icon each render: the tab may have navigated, and a stale
  // favicon is worse than none.
  const top = card.querySelector('.current-top');
  top.querySelectorAll('.icon-slot').forEach((n) => n.remove());
  top.prepend(buildIcon(tab.favIconUrl, label));

  $('current-title').textContent = label;
  $('current-host').textContent = host;
  $('enabled').checked = capturing;

  const count = tab.count || 0;
  const foot = $('current-foot');
  const countEl = $('current-count');
  const agoEl = $('current-ago');
  const arrow = $('current-arrow');

  // While the app is away the warning card and the buffer line carry the
  // message, and the tab card shrinks to just the switch — as in the reference.
  foot.hidden = !state.appRunning;
  if (foot.hidden) {
    foot.classList.remove('actionable');
    foot.onclick = null;
    return;
  }

  if (!capturing) {
    countEl.textContent = '';
    agoEl.textContent = 'Запросы этой вкладки не пишутся. Включите, и они появятся в приложении.';
    arrow.hidden = true;
    foot.classList.remove('actionable');
    // Prose, not columns: it wraps instead of being cut with an ellipsis.
    foot.classList.add('text');
    foot.onclick = null;
    return;
  }

  foot.classList.remove('text');

  if (count === 0) {
    countEl.textContent = '';
    agoEl.textContent = 'Ждём первый запрос…';
    arrow.hidden = true;
    foot.classList.remove('actionable');
    foot.onclick = null;
    return;
  }

  countEl.textContent = requestsLabel(count);
  agoEl.textContent = agoLabel(tab.lastAt);
  arrow.hidden = false;
  foot.classList.add('actionable');
  foot.onclick = () => chrome.runtime.sendMessage({ type: 'openApp', tabId: tab.tabId });
}

function renderWarning(state) {
  const warn = $('warn-card');
  warn.hidden = state.appRunning;
  if (!state.appRunning) $('warn-port').textContent = String(state.port || DEFAULT_PORT);

  const line = $('buffered-line');
  const buffered = state.bufferedCount || 0;
  line.hidden = state.appRunning || buffered === 0;
  if (!line.hidden) {
    $('buffered-text').textContent = `${requestsLabel(buffered)} в буфере, отдадим после запуска`;
  }
}

function renderOtherTabs(state) {
  const section = $('more-section');
  const list = $('more-list');
  list.textContent = '';

  const others = (state.capturedTabs || []).filter((t) => t.tabId !== state.activeTabId);
  if (others.length === 0) {
    section.hidden = true;
    return;
  }
  section.hidden = false;
  for (const t of others) list.appendChild(buildMoreRow(t));
}

function renderOpenButton(state) {
  const btn = $('open-app');
  // Loud when there is a reason to press it; quiet when the app is already up
  // and nothing is being recorded.
  const secondary = state.appRunning && !state.capturing;
  btn.className = 'open-app' + (secondary ? ' secondary' : '');
}

const SETTING_INPUTS = ['set-rememberTabs', 'set-xhrOnly', 'set-clearOnExit'];

function renderSettings(state) {
  // Never fight the user for a field they're typing in.
  const active = document.activeElement;
  const busy = active && active.closest && active.closest('.view-settings');

  if (!busy) {
    $('port').value = state.port || DEFAULT_PORT;
    for (const id of SETTING_INPUTS) {
      const key = id.replace('set-', '');
      $(id).checked = Boolean(state.settings && state.settings[key]);
    }
  }
}

function render(state) {
  renderHeader(state);
  renderCurrentTab(state);
  renderWarning(state);
  renderOtherTabs(state);
  renderOpenButton(state);
  renderSettings(state);
}

async function refresh() {
  try {
    render(await getState());
  } catch (_) {
    // The worker can be mid-restart; the next poll will pick it up.
  }
}

// --- interactions -----------------------------------------------------------

$('enabled').addEventListener('change', async (e) => {
  await chrome.runtime.sendMessage(e.target.checked ? { type: 'startCapture' } : { type: 'stopCapture' });
  await refresh();
});

$('open-app').addEventListener('click', () => chrome.runtime.sendMessage({ type: 'openApp' }));
$('warn-launch').addEventListener('click', () => chrome.runtime.sendMessage({ type: 'openApp' }));

$('open-settings').addEventListener('click', () => document.body.classList.add('settings'));
$('close-settings').addEventListener('click', () => document.body.classList.remove('settings'));

$('port').addEventListener('change', async (e) => {
  const p = parseInt(e.target.value, 10);
  if (p > 0 && p < 65536) await chrome.runtime.sendMessage({ type: 'setPort', port: p });
  await refresh();
});

for (const id of SETTING_INPUTS) {
  $(id).addEventListener('change', async (e) => {
    await chrome.runtime.sendMessage({
      type: 'setSetting',
      key: id.replace('set-', ''),
      value: e.target.checked,
    });
    await refresh();
  });
}

let pollTimer = null;
function startPolling() {
  stopPolling();
  pollTimer = setInterval(refresh, 2500);
}
function stopPolling() {
  if (pollTimer) clearInterval(pollTimer);
  pollTimer = null;
}

// Written once at load rather than when the gear is pressed, so the footer is
// never blank.
$('settings-version').textContent = 'Версия ' + chrome.runtime.getManifest().version;

refresh().then(startPolling);
window.addEventListener('unload', stopPolling);
