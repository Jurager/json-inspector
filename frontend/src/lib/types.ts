// Unified shape for a request/response, whether it originated from the manual
// builder or was captured by the browser extension.
export interface RequestRecord {
  id: string
  method: string
  url: string
  requestHeaders: Record<string, string>
  requestBody: string
  status: number
  statusText: string
  responseHeaders: Record<string, string>
  responseBody: string
  durationMs: number
  startedAt: number
  source: 'manual' | 'browser'
  contentType?: string
  error?: string
  tabTitle?: string
  tabURL?: string
  tabId?: number
  favIconUrl?: string
}
