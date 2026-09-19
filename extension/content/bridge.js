(() => {
  if (window.__jsonInspectorBridge) {
    return;
  }

  window.__jsonInspectorBridge = true;

  window.addEventListener('message', (event) => {
    // Only this page's own hook may report. Any frame on a captured page can post a message with the
    // marker below, and a capture that came from one of them would be filed as this tab's — under
    // this tab's title and address, which is what makes it worth forging.
    if (event.source !== window) {
      return;
    }

    const data = event.data;

    if (
        data?.__jsonInspector !== '__JSON_INSPECTOR_CAPTURE__' ||
        data.kind !== 'request'
    ) {
      return;
    }

    // The extension was reloaded or updated under this page: this context is dead, and every call
    // from it throws — once per request, on a page nobody is looking at. The hook in the page cannot
    // be switched off from here (it lives in the page's own world, and this is the isolated one), so
    // the failure is swallowed and capture ends when the page is reloaded.
    try {
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
            startedAt: data.startedAt,
            tabTitle: document.title,
            tabURL: location.href,
          })
          .catch(() => {});
    } catch {
      // Swallowed on purpose, and for the reason above.
    }
  });
})();
