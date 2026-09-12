// A request/response from either the manual builder or the browser extension.
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
  dnsMs?: number
  connectMs?: number
  tlsMs?: number
  waitMs?: number
  downloadMs?: number
  tabTitle?: string
  tabURL?: string
  tabId?: number
  favIconUrl?: string
}
