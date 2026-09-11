const DEFAULT_PORT = 38761;

async function getState() {
  return chrome.runtime.sendMessage({
    type: 'getState',
  });
}

function $(id) {
  return document.getElementById(id);
}

async function openApp(tabId) {
  try {
    const res = await chrome.runtime.sendMessage({
      type: 'focusApp',
      tabId,
    });

    if (res?.ok) {
      window.close();
      return;
    }
  } catch {}

  const query =
      tabId != null
          ? `?tab=${encodeURIComponent(tabId)}`
          : '';

  window.location.href = `json-inspector://open${query}`;
}

function hostnameOf(url) {
  try {
    return new URL(url).hostname;
  } catch {
    return '';
  }
}

function plural(n, one, few, many) {
  const m10 = n % 10;
  const m100 = n % 100;

  if (m10 === 1 && m100 !== 11) {
    return `${n} ${one}`;
  }

  if (
      m10 >= 2 &&
      m10 <= 4 &&
      (m100 < 12 || m100 > 14)
  ) {
    return `${n} ${few}`;
  }

  return `${n} ${many}`;
}

const requestsLabel = (n) =>
    plural(n, 'запрос', 'запроса', 'запросов');

function agoLabel(at) {
  if (!at) {
    return '';
  }

  const sec = Math.max(
      0,
      Math.round((Date.now() - at) / 1000)
  );

  if (sec < 60) {
    return `последний ${sec} с назад`;
  }

  const min = Math.round(sec / 60);

  if (min < 60) {
    return `последний ${min} мин назад`;
  }

  return `последний ${Math.round(min / 60)} ч назад`;
}

function buildIcon(favIconUrl, label) {
  const wrap = document.createElement('span');
  wrap.className = 'icon-slot';

  const initial =
      (label || '?').trim().charAt(0) || '?';

  const avatar = document.createElement('span');
  avatar.className = 'avatar';
  avatar.textContent = initial;
  avatar.hidden = Boolean(favIconUrl);

  if (favIconUrl) {
    const img = document.createElement('img');

    img.className = 'favicon';
    img.src = favIconUrl;
    img.alt = '';

    img.addEventListener('error', () => {
      img.classList.add('hidden');
      avatar.hidden = false;
    });

    wrap.appendChild(img);
  }

  wrap.appendChild(avatar);

  return wrap;
}

function buildMoreRow(tab) {
  const row = document.createElement('div');
  row.className = 'more-row';

  const host = hostnameOf(tab.url);

  row.appendChild(
      buildIcon(
          tab.favIconUrl,
          tab.title || host
      )
  );

  const text = document.createElement('div');
  text.className = 'more-text';

  const title = document.createElement('div');
  title.className = 'more-title';
  title.textContent =
      tab.title || host || 'Вкладка';

  const hostEl = document.createElement('div');
  hostEl.className = 'more-host mono';
  hostEl.textContent = host;

  text.append(title, hostEl);
  row.appendChild(text);

  const count = document.createElement('span');
  count.className = 'more-count';
  count.textContent = String(tab.count || 0);
  row.appendChild(count);

  const stop = document.createElement('button');

  stop.className = 'more-stop';
  stop.title = 'Остановить перехват этой вкладки';
  stop.innerHTML =
      '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><path d="M6 6l12 12M18 6 6 18"/></svg>';

  stop.addEventListener('click', async (event) => {
    event.stopPropagation();

    stop.disabled = true;

    try {
      await chrome.runtime.sendMessage({
        type: 'stopCaptureTab',
        tabId: tab.tabId,
      });

      await refresh();
    } finally {
      stop.disabled = false;
    }
  });

  row.appendChild(stop);

  row.addEventListener('click', () => {
    openApp(tab.tabId);
  });

  return row;
}

function renderHeader(state) {
  const dot = $('status-dot');
  const text = $('status-text');
  const port = $('status-port');

  if (state.appRunning) {
    dot.className = 'dot ok';
    text.textContent = 'Подключено';
    port.textContent = ':' + (
        state.port || DEFAULT_PORT
    );
    return;
  }

  dot.className = 'dot warn';
  text.textContent = 'Приложение не запущено';
  port.textContent = '';
}

function renderCurrentTab(state) {
  const card = $('current-card');
  const tab = state.currentTab;
  const capturing = Boolean(
      tab?.capturing
  );

  card.className =
      'card current ' +
      (capturing ? 'on' : 'off') +
      (state.appRunning ? '' : ' muted');

  if (!tab) {
    $('current-title').textContent =
        'Нет активной вкладки';

    $('current-host').textContent = '';
    $('enabled').checked = false;
    $('current-count').textContent = '';
    $('current-ago').textContent = '';
    $('current-arrow').hidden = true;
    $('current-foot').classList.remove(
        'actionable',
        'text'
    );

    return;
  }

  const host = hostnameOf(tab.url);
  const label =
      tab.title || host || 'Вкладка';

  const top = card.querySelector(
      '.current-top'
  );

  top
      .querySelectorAll('.icon-slot')
      .forEach((node) => node.remove());

  top.prepend(
      buildIcon(tab.favIconUrl, label)
  );

  $('current-title').textContent = label;
  $('current-host').textContent = host;
  $('enabled').checked = capturing;

  const count = tab.count || 0;
  const foot = $('current-foot');
  const countEl = $('current-count');
  const agoEl = $('current-ago');
  const arrow = $('current-arrow');

  foot.hidden = !state.appRunning;

  if (foot.hidden) {
    foot.classList.remove(
        'actionable',
        'text'
    );
    foot.onclick = null;
    return;
  }

  if (!capturing) {
    countEl.textContent = '';

    agoEl.textContent =
        'Запросы этой вкладки не пишутся. Включите, и они появятся в приложении.';

    arrow.hidden = true;

    foot.classList.remove('actionable');
    foot.classList.add('text');
    foot.onclick = null;

    return;
  }

  foot.classList.remove('text');

  if (count === 0) {
    countEl.textContent = '';
    agoEl.textContent =
        'Ждём первый запрос…';

    arrow.hidden = true;
    foot.classList.remove('actionable');
    foot.onclick = null;

    return;
  }

  countEl.textContent =
      requestsLabel(count);

  agoEl.textContent =
      agoLabel(tab.lastAt);

  arrow.hidden = false;

  foot.classList.add('actionable');

  foot.onclick = () => {
    openApp(tab.tabId);
  };
}

function renderWarning(state) {
  const warn = $('warn-card');

  warn.hidden = state.appRunning;

  if (!state.appRunning) {
    $('warn-port').textContent =
        String(state.port || DEFAULT_PORT);
  }

  const line = $('buffered-line');
  const buffered =
      state.bufferedCount || 0;

  line.hidden =
      state.appRunning || buffered === 0;

  if (!line.hidden) {
    $('buffered-text').textContent =
        `${requestsLabel(buffered)} в буфере, отдадим после запуска`;
  }
}

function renderOtherTabs(state) {
  const section = $('more-section');
  const list = $('more-list');

  list.textContent = '';

  const others = (
      state.capturedTabs || []
  ).filter(
      (tab) => tab.tabId !== state.activeTabId
  );

  section.hidden = others.length === 0;

  if (section.hidden) {
    return;
  }

  for (const tab of others) {
    list.appendChild(
        buildMoreRow(tab)
    );
  }
}

function renderOpenButton(state) {
  const button = $('open-app');

  const secondary =
      state.appRunning &&
      !state.capturing;

  button.className =
      'open-app' +
      (secondary ? ' secondary' : '');
}

const SETTING_INPUTS = [
  'set-rememberTabs',
  'set-xhrOnly',
  'set-clearOnExit',
];

function renderSettings(state) {
  const active = document.activeElement;
  const settingsView =
      active?.closest?.('.view-settings');

  if (settingsView) {
    return;
  }

  $('port').value =
      state.port || DEFAULT_PORT;

  for (const id of SETTING_INPUTS) {
    const key = id.replace('set-', '');

    $(id).checked = Boolean(
        state.settings?.[key]
    );
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
    const state = await getState();
    render(state);
  } catch {}
}

let actionInProgress = false;

$('enabled').addEventListener(
    'change',
    async (event) => {
      if (actionInProgress) {
        return;
      }

      actionInProgress = true;
      event.target.disabled = true;

      try {
        await chrome.runtime.sendMessage(
            event.target.checked
                ? { type: 'startCapture' }
                : { type: 'stopCapture' }
        );

        await refresh();
      } finally {
        actionInProgress = false;
        event.target.disabled = false;
      }
    }
);

$('open-app').addEventListener(
    'click',
    () => openApp()
);

$('warn-launch').addEventListener(
    'click',
    () => openApp()
);

$('open-settings').addEventListener(
    'click',
    () => {
      document.body.classList.add(
          'settings'
      );
    }
);

$('close-settings').addEventListener(
    'click',
    () => {
      document.body.classList.remove(
          'settings'
      );
    }
);

$('port').addEventListener(
    'change',
    async (event) => {
      const port = parseInt(
          event.target.value,
          10
      );

      if (port > 0 && port < 65536) {
        try {
          await chrome.runtime.sendMessage({
            type: 'setPort',
            port,
          });
        } catch {}
      }

      await refresh();
    }
);

for (const id of SETTING_INPUTS) {
  $(id).addEventListener(
      'change',
      async (event) => {
        try {
          await chrome.runtime.sendMessage({
            type: 'setSetting',
            key: id.replace('set-', ''),
            value: event.target.checked,
          });
        } catch {}

        await refresh();
      }
  );
}

let pollTimer = null;

function startPolling() {
  stopPolling();

  pollTimer = setInterval(
      refresh,
      2500
  );
}

function stopPolling() {
  if (pollTimer !== null) {
    clearInterval(pollTimer);
  }

  pollTimer = null;
}

$('settings-version').textContent =
    'Версия ' +
    chrome.runtime.getManifest().version;

refresh().finally(startPolling);

window.addEventListener(
    'unload',
    stopPolling
);