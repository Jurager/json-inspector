const DEFAULT_PORT = 38761;

async function getState() {
  return await chrome.runtime.sendMessage({ type: 'getState' });
}

async function init() {
  const state = await getState();
  const capturingThisTab = state.capturing && state.captureTabId === state.activeTabId;
  document.getElementById('enabled').checked = capturingThisTab;
  document.getElementById('port').value = state.port || DEFAULT_PORT;
  updateStatus(capturingThisTab, state.appRunning);
}

function updateStatus(capturing, appRunning) {
  const appEl = document.getElementById('app-status');
  appEl.textContent = appRunning ? '✓ Приложение запущено' : '✗ Приложение не запущено';
  appEl.className = 'status ' + (appRunning ? 'ok' : 'warn');

  const capEl = document.getElementById('cap-status');
  if (!capturing) {
    capEl.textContent = 'Перехват выключен для этой вкладки.';
    capEl.className = 'status';
  } else if (!appRunning) {
    capEl.textContent = 'Перехват включён, но приложение недоступно — запросы не дойдут.';
    capEl.className = 'status warn';
  } else {
    capEl.textContent = 'Перехват активен — запросы идут в приложение.';
    capEl.className = 'status ok';
  }
}

document.getElementById('enabled').addEventListener('change', async (e) => {
  if (e.target.checked) {
    await chrome.runtime.sendMessage({ type: 'startCapture' });
  } else {
    await chrome.runtime.sendMessage({ type: 'stopCapture' });
  }
  const state = await getState();
  updateStatus(state.capturing, state.appRunning);
});

document.getElementById('port').addEventListener('change', async (e) => {
  const p = parseInt(e.target.value, 10);
  if (p > 0 && p < 65536) await chrome.runtime.sendMessage({ type: 'setPort', port: p });
});

init();
