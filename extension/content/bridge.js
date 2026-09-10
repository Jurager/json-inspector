(() => {
  if (window.__jsonInspectorBridge) return;
  window.__jsonInspectorBridge = true;

  window.addEventListener('message', (event) => {
    const data = event.data;
    if (!data || data.__jsonInspector !== '__JSON_INSPECTOR_CAPTURE__' || data.kind !== 'request') return;

    chrome.runtime.sendMessage({
      type: 'captured',
      method: data.method,
      url: data.url,
      requestHeaders: data.requestHeaders,
      requestBody: data.requestBody,
      status: data.status,
      statusText: data.statusText,
      responseHeaders: data.responseHeaders,
      responseBody: data.responseBody,
      durationMs: data.durationMs,
      tabTitle: document.title,
      tabURL: location.href,
    }).catch(() => {});
  });
})();
