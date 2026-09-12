(() => {
  if (window.__jsonInspectorBridge) {
    return;
  }

  window.__jsonInspectorBridge = true;

  window.addEventListener('message', ({ data }) => {
    if (
        data?.__jsonInspector !== '__JSON_INSPECTOR_CAPTURE__' ||
        data.kind !== 'request'
    ) {
      return;
    }

    chrome.runtime
        .sendMessage({
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
          hasTiming: data.hasTiming,
          waitMs: data.waitMs,
          downloadMs: data.downloadMs,
          tabTitle: document.title,
          tabURL: location.href,
        })
        .catch(() => {});
  });
})();