(() => {
  const MESSAGE_MARKER = '__JSON_INSPECTOR_CAPTURE__';

  // Prevent double-patching if the script is injected more than once —
  // just re-enable capture on the existing hook.
  if (window.__jsonInspectorHook) {
    window.__jsonInspectorEnabled = true;
    return;
  }
  window.__jsonInspectorHook = true;
  window.__jsonInspectorEnabled = true;

  const isCaptureEnabled = () => window.__jsonInspectorEnabled === true;

  /**
   * Normalizes any of the header shapes we might encounter (Headers,
   * array-of-pairs, plain object) into a plain { name: value } object.
   */
  function serializeHeaders(headers) {
    const out = {};
    if (!headers) return out;

    if (typeof headers.forEach === 'function') {
      headers.forEach((value, key) => { out[key] = value; });
    } else if (Array.isArray(headers)) {
      for (const [key, value] of headers) out[key] = value;
    } else {
      for (const key of Object.keys(headers)) out[key] = headers[key];
    }
    return out;
  }

  /** Parses the raw string from XHR.getAllResponseHeaders() into an object. */
  function parseRawResponseHeaders(rawHeaderString) {
    const out = {};
    if (!rawHeaderString) return out;

    for (const line of rawHeaderString.trim().split(/\r?\n/)) {
      const separatorIndex = line.indexOf(':');
      if (separatorIndex <= 0) continue;
      const name = line.slice(0, separatorIndex).trim();
      const value = line.slice(separatorIndex + 1).trim();
      out[name] = value;
    }
    return out;
  }

  /** Best-effort stringification of a fetch/XHR request body. */
  function bodyToString(body) {
    if (body == null) return '';
    if (typeof body === 'string') return body;
    if (body instanceof URLSearchParams) return body.toString();
    if (body instanceof ArrayBuffer || ArrayBuffer.isView(body)) return '[binary body]';
    if (body instanceof Blob) return '[blob body]';
    if (body instanceof FormData) return '[form data]';

    try {
      return JSON.stringify(body);
    } catch (_err) {
      return String(body);
    }
  }

  function postToContentScript(payload) {
    try {
      window.postMessage({ __jsonInspector: MESSAGE_MARKER, ...payload }, '*');
    } catch (_err) {
      // Ignore postMessage failures (e.g. detached window during navigation).
    }
  }

  function capture({
    method,
    url,
    requestHeaders = {},
    requestBody = '',
    status = 0,
    statusText = '',
    responseHeaders = {},
    responseBody = '',
    durationMs = 0,
  }) {
    if (!isCaptureEnabled()) return;
    postToContentScript({
      kind: 'request',
      method,
      url,
      requestHeaders,
      requestBody,
      status,
      statusText,
      responseHeaders,
      responseBody,
      durationMs,
    });
  }

  installFetchHook();
  installXhrHook();

  // ---------------------------------------------------------------------

  function installFetchHook() {
    const originalFetch = window.fetch;
    if (typeof originalFetch !== 'function') return;

    window.fetch = function patchedFetch(input, init = {}) {
      const startedAt = Date.now();
      const promise = originalFetch.call(this, input, init);

      if (!isCaptureEnabled()) return promise;

      const url = typeof input === 'string'
        ? input
        : (input && typeof input.url === 'string' ? input.url : String(input));
      const method = (init.method || (input && input.method) || 'GET').toUpperCase();
      const requestHeaders = serializeHeaders(init.headers || (input && input.headers));
      const requestBody = bodyToString(init.body);

      promise.then(
        (response) => {
          const durationMs = Date.now() - startedAt;
          const responseHeaders = serializeHeaders(response.headers);

          response.clone().text().then(
            (responseBody) => capture({
              method, url, requestHeaders, requestBody,
              status: response.status, statusText: response.statusText,
              responseHeaders, responseBody, durationMs,
            }),
            () => capture({
              method, url, requestHeaders, requestBody,
              status: response.status, statusText: response.statusText,
              responseHeaders, durationMs,
            })
          );
        },
        (error) => capture({
          method, url, requestHeaders, requestBody,
          status: 0, statusText: String(error && error.message || error),
          durationMs: Date.now() - startedAt,
        })
      );

      return promise;
    };
  }

  function installXhrHook() {
    const originalOpen = XMLHttpRequest.prototype.open;
    const originalSend = XMLHttpRequest.prototype.send;
    const originalSetRequestHeader = XMLHttpRequest.prototype.setRequestHeader;

    XMLHttpRequest.prototype.open = function patchedOpen(method, url, ...rest) {
      this.__ji = {
        method: String(method).toUpperCase(),
        url: String(url),
        requestHeaders: {},
        startedAt: Date.now(),
      };
      return originalOpen.call(this, method, url, ...rest);
    };

    XMLHttpRequest.prototype.setRequestHeader = function patchedSetRequestHeader(name, value) {
      if (this.__ji) this.__ji.requestHeaders[name] = value;
      return originalSetRequestHeader.call(this, name, value);
    };

    XMLHttpRequest.prototype.send = function patchedSend(body, ...rest) {
      const state = this.__ji;

      if (state && isCaptureEnabled()) {
        const requestBody = bodyToString(body);

        const onLoad = () => capture({
          method: state.method,
          url: state.url,
          requestHeaders: state.requestHeaders,
          requestBody,
          status: this.status,
          statusText: this.statusText,
          responseHeaders: parseRawResponseHeaders(
            typeof this.getAllResponseHeaders === 'function' ? this.getAllResponseHeaders() : ''
          ),
          responseBody: typeof this.responseText === 'string' ? this.responseText : '',
          durationMs: Date.now() - state.startedAt,
        });

        const onError = () => capture({
          method: state.method,
          url: state.url,
          requestHeaders: state.requestHeaders,
          requestBody,
          status: 0,
          statusText: 'network error',
          durationMs: Date.now() - state.startedAt,
        });

        this.addEventListener('load', onLoad, { once: true });
        this.addEventListener('error', onError, { once: true });
      }

      return originalSend.call(this, body, ...rest);
    };
  }
})();