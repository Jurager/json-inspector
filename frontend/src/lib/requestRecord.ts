import type { CookieRow } from './cookies'

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
  // Only meaningful for a captured (browser) record — a manual one always has the phases
  // above from Go's own httptrace, this just isn't set for it. See TimingsTab's hasDetail.
  hasTiming?: boolean
  // The structured request-side jar "Cookies" edits in "Запрос" mode — manual only; a captured
  // record has no draft of its own to have edited one for.
  requestCookies?: CookieRow[]
  tabTitle?: string
  tabURL?: string
  tabId?: number
  favIconUrl?: string
}
