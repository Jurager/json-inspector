(() => {
  const MESSAGE_MARKER = '__JSON_INSPECTOR_CAPTURE__';
  const MAX_BODY_CHARS = 2 * 1024 * 1024;
  const HOOK_VERSION = 3;

  window.__jsonInspectorLastInjection = HOOK_VERSION;

  if (window.__jsonInspectorHook) {
    window.__jsonInspectorEnabled = true;
    return;
  }

  window.__jsonInspectorHook = true;
  window.__jsonInspectorHookVersion = HOOK_VERSION;
  window.__jsonInspectorEnabled = true;

  const isCaptureEnabled = () => window.__jsonInspectorEnabled === true;

  function isBinaryContentType(contentType) {
    if (!contentType) {
      return false;
    }

    const type = contentType.split(';')[0].trim().toLowerCase();

    return (
        type.startsWith('image/') ||
        type.startsWith('audio/') ||
        type.startsWith('video/') ||
        type.startsWith('font/') ||
        /^application\/(pdf|zip|gzip|octet-stream|wasm|x-binary|x-compressed)$/.test(type)
    );
  }

  function limitBody(body) {
    if (body.length <= MAX_BODY_CHARS) {
      return body;
    }

    return `${body.slice(0, MAX_BODY_CHARS)}\n…[обрезано до 2 МБ]`;
  }

  function serializeHeaders(headers) {
    if (!headers) {
      return {};
    }

    const result = {};

    if (typeof headers.forEach === 'function') {
      headers.forEach((value, key) => {
        result[key] = value;
      });

      return result;
    }

    if (Array.isArray(headers)) {
      for (const [key, value] of headers) {
        result[key] = value;
      }

      return result;
    }

    for (const key of Object.keys(headers)) {
      result[key] = headers[key];
    }

    return result;
  }

  function parseResponseHeaders(headers) {
    if (!headers) {
      return {};
    }

    const result = {};

    for (const line of headers.trim().split(/\r?\n/)) {
      const separator = line.indexOf(':');

      if (separator <= 0) {
        continue;
      }

      const name = line.slice(0, separator).trim();
      const value = line.slice(separator + 1).trim();

      result[name] = value;
    }

    return result;
  }

  function bodyToString(body) {
    if (body == null) {
      return '';
    }

    if (typeof body === 'string') {
      return body;
    }

    if (body instanceof URLSearchParams) {
      return body.toString();
    }

    if (body instanceof ArrayBuffer || ArrayBuffer.isView(body)) {
      return '[binary body]';
    }

    if (body instanceof Blob) {
      return '[blob body]';
    }

    if (body instanceof FormData) {
      return '[form data]';
    }

    try {
      return JSON.stringify(body);
    } catch {
      return String(body);
    }
  }

  function postToContentScript(payload) {
    try {
      window.postMessage(
          {
            __jsonInspector: MESSAGE_MARKER,
            ...payload,
          },
          '*'
      );
    } catch {
      // Ignore postMessage failures during navigation.
    }
  }

  function capture(data) {
    if (!isCaptureEnabled()) {
      return;
    }

    postToContentScript({
      kind: 'request',
      ...data,
    });
  }

  function getBinaryResponseBody(contentType) {
    return `[binary response: ${contentType.split(';')[0]}]`;
  }

  function phases(startedAt, headersAt, bodyAt) {
    return {
      hasTiming: true,
      waitMs: Math.max(0, Math.round(headersAt - startedAt)),
      downloadMs: Math.max(0, Math.round(bodyAt - headersAt)),
    };
  }

  function installFetchHook() {
    const originalFetch = window.fetch;

    if (typeof originalFetch !== 'function') {
      return;
    }

    window.fetch = function patchedFetch(input, init = {}) {
      const startedAt = Date.now();
      const startedPerf = performance.now();
      const promise = originalFetch.call(this, input, init);

      if (!isCaptureEnabled()) {
        return promise;
      }

      const url =
          typeof input === 'string'
              ? input
              : input?.url || String(input);

      const method = (
          init.method ||
          input?.method ||
          'GET'
      ).toUpperCase();

      const requestHeaders = serializeHeaders(
          init.headers || input?.headers
      );

      const requestBody = bodyToString(init.body);

      promise.then(
          async (response) => {
            // Resolved means the headers are in — see phases().
            const headersPerf = performance.now();
            const responseHeaders = serializeHeaders(response.headers);
            const contentType = response.headers.get('content-type') || '';

            if (isBinaryContentType(contentType)) {
              // The body is deliberately not read here, so its download time is unknown and
              // the phases are left out rather than reported as zero.
              capture({
                method,
                url,
                requestHeaders,
                requestBody,
                status: response.status,
                statusText: response.statusText,
                responseHeaders,
                responseBody: getBinaryResponseBody(contentType),
                durationMs: Date.now() - startedAt,
              });

              return;
            }

            let responseBody = '';

            try {
              responseBody = limitBody(await response.clone().text());
            } catch {
              // Keep empty response body when it cannot be read.
            }

            // Measured after the body, so the total stays the whole round trip and the two
            // phases above add up to it — otherwise "Всего" would silently mean "до заголовков".
            capture({
              method,
              url,
              requestHeaders,
              requestBody,
              status: response.status,
              statusText: response.statusText,
              responseHeaders,
              responseBody,
              durationMs: Date.now() - startedAt,
              ...phases(startedPerf, headersPerf, performance.now()),
            });
          },
          (error) => {
            capture({
              method,
              url,
              requestHeaders,
              requestBody,
              status: 0,
              statusText: String(error?.message || error),
              durationMs: Date.now() - startedAt,
            });
          }
      );

      return promise;
    };
  }

  function installXhrHook() {
    const originalOpen = XMLHttpRequest.prototype.open;
    const originalSend = XMLHttpRequest.prototype.send;
    const originalSetRequestHeader =
        XMLHttpRequest.prototype.setRequestHeader;

    XMLHttpRequest.prototype.open = function patchedOpen(
        method,
        url,
        ...rest
    ) {
      this.__ji = {
        method: String(method).toUpperCase(),
        url: String(url),
        requestHeaders: {},
        startedAt: Date.now(),
      };

      return originalOpen.call(this, method, url, ...rest);
    };

    XMLHttpRequest.prototype.setRequestHeader =
        function patchedSetRequestHeader(name, value) {
          if (this.__ji) {
            this.__ji.requestHeaders[name] = value;
          }

          return originalSetRequestHeader.call(this, name, value);
        };

    XMLHttpRequest.prototype.send = function patchedSend(body, ...rest) {
      const state = this.__ji;

      if (state && isCaptureEnabled()) {
        const requestBody = bodyToString(body);
        const startedPerf = performance.now();
        // readyState 2 is HEADERS_RECEIVED — the same instant fetch()'s promise resolves at.
        let headersPerf = 0;

        const captureResponse = () => {
          const contentType =
              this.getResponseHeader?.('content-type') || '';

          let responseBody = '';
          let timing = null;

          if (isBinaryContentType(contentType)) {
            responseBody = getBinaryResponseBody(contentType);
          } else if (typeof this.responseText === 'string') {
            responseBody = limitBody(this.responseText);
            // Only a body that was actually read has a download time to report.
            if (headersPerf) timing = phases(startedPerf, headersPerf, performance.now());
          }

          capture({
            method: state.method,
            url: state.url,
            requestHeaders: state.requestHeaders,
            requestBody,
            status: this.status,
            statusText: this.statusText,
            responseHeaders: parseResponseHeaders(
                this.getAllResponseHeaders?.() || ''
            ),
            responseBody,
            durationMs: Date.now() - state.startedAt,
            ...(timing ?? {}),
          });
        };

        const captureError = () => {
          capture({
            method: state.method,
            url: state.url,
            requestHeaders: state.requestHeaders,
            requestBody,
            status: 0,
            statusText: 'network error',
            durationMs: Date.now() - state.startedAt,
          });
        };

        this.addEventListener('readystatechange', () => {
          if (!headersPerf && this.readyState >= 2) {
            headersPerf = performance.now();
          }
        });
        this.addEventListener('load', captureResponse, { once: true });
        this.addEventListener('error', captureError, { once: true });
      }

      return originalSend.call(this, body, ...rest);
    };
  }

  installFetchHook();
  installXhrHook();
})();