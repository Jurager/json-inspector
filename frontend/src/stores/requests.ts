import { defineStore } from 'pinia'
import type { RequestRecord } from '../lib/types'
import { useEnvironmentsStore } from './environments'

let counter = 0
function nextId(): string {
  counter += 1
  return `req-${Date.now()}-${counter}`
}

// The status bar's source of truth for the extension's state, fed by its capture-state messages.
interface CaptureState {
  connected: boolean
  recording: boolean
  tabs: number
}

interface KeyValueRow {
  name: string
  value: string
  enabled: boolean
}

type AuthType = 'none' | 'bearer' | 'basic' | 'oauth2'

// In the store rather than RequestBuilder's refs so "Открыть в «Запросе»" can fill the command line.
interface DraftState {
  method: string
  url: string
  params: KeyValueRow[]
  headers: KeyValueRow[]
  auth: { type: AuthType; token: string }
  body: string
}

export const useRequestsStore = defineStore('requests', {
  state: () => ({
    requests: [] as RequestRecord[],
    manualId: null as string | null,
    browserId: null as string | null,
    activeView: 'request' as 'request' | 'browser' | 'collections',
    capturing: false,
    unreadCount: 0,
    loading: false,
    capture: { connected: false, recording: false, tabs: 0 } as CaptureState,
    draft: {
      method: 'GET',
      url: '',
      params: [] as KeyValueRow[],
      headers: [{ name: 'Accept', value: 'application/vnd.api+json', enabled: true }] as KeyValueRow[],
      auth: { type: 'none' as AuthType, token: '' },
      body: '',
    } as DraftState,
    openChip: null as 'params' | 'headers' | 'auth' | 'body' | null,
    // A tab the extension asked us to show (json-inspector://open?tab=42); the browser
    // list expands that group and selects its newest request.
    focusTabId: null as number | null,
    // Bumped by "Открыть в «Запросе»" so RequestBuilder focuses its URL field after remounting.
    focusUrlTick: 0,
    inspector: { open: false, path: null as string | null, width: 300 },
  }),
  getters: {
    manualSelected(state): RequestRecord | null {
      return state.requests.find((r) => r.id === state.manualId) ?? null
    },
    browserSelected(state): RequestRecord | null {
      return state.requests.find((r) => r.id === state.browserId) ?? null
    },
    // Only what would actually be sent counts — a disabled row is not part of the
    // request, so an unknown token in one must not block the send.
    missingVars(state): string[] {
      const envs = useEnvironmentsStore()
      const parts = [
        state.draft.url,
        ...state.draft.params.filter((p) => p.enabled).map((p) => `${p.name}\n${p.value}`),
        ...state.draft.headers.filter((h) => h.enabled).map((h) => `${h.name}\n${h.value}`),
        state.draft.body,
        state.draft.auth.token,
      ]
      return envs.missingIn(parts.join('\n'))
    },
  },
  actions: {
    add(record: Omit<RequestRecord, 'id' | 'startedAt'>) {
      const full: RequestRecord = {
        ...record,
        id: nextId(),
        startedAt: Date.now(),
      }
      this.requests.unshift(full)
      this.manualId = full.id
    },
    addCaptured(record: Omit<RequestRecord, 'id' | 'source' | 'startedAt'>) {
      const full: RequestRecord = {
        ...record,
        id: nextId(),
        startedAt: Date.now(),
        source: 'browser',
      }
      this.requests.unshift(full)
      if (this.activeView !== 'browser') {
        this.unreadCount += 1
      }
    },
    hydrate(requests: RequestRecord[]) {
      this.requests = requests
      this.manualId = null
      this.browserId = null
      this.unreadCount = 0
    },
    selectManual(id: string) {
      this.manualId = id
      // Loads the entry into the command line too, so the editor and the shown response
      // stay in sync — the same as "Открыть в «Запросе»", minus the view switch.
      const r = this.requests.find((x) => x.id === id)
      if (r) this.loadDraft(r)
    },
    selectBrowser(id: string) {
      this.browserId = id
    },
    markBrowserRead() {
      this.unreadCount = 0
    },
    // The panel does the landing — only it knows whether that tab has produced anything yet.
    focusBrowserTab(tabId: number) {
      this.activeView = 'browser'
      this.unreadCount = 0
      this.focusTabId = tabId
    },
    setCaptureState(partial: Partial<CaptureState>) {
      this.capture = { ...this.capture, ...partial }
    },
    setOpenChip(chip: 'params' | 'headers' | 'auth' | 'body' | null) {
      this.openChip = chip
    },
    requestFocusUrl() {
      this.focusUrlTick++
    },
    // Global search has no index yet, so it points at the command line; ⌘K and the titlebar button come through here.
    focusSearch() {
      this.activeView = 'request'
      this.requestFocusUrl()
    },
    setInspector(partial: Partial<{ open: boolean; path: string | null; width: number }>) {
      this.inspector = { ...this.inspector, ...partial }
    },
    // Without sending it — "Открыть в «Запросе»" turns a read-only record back into an editable draft.
    loadDraft(record: Pick<RequestRecord, 'method' | 'url' | 'requestHeaders' | 'requestBody'>) {
      this.draft.method = record.method
      this.setUrl(record.url)
      this.draft.headers = Object.entries(record.requestHeaders).map(([name, value]) => ({
        name,
        value,
        enabled: true,
      }))
      this.draft.body = record.requestBody
    },
    // The URL → params direction: a manual URL edit is the source of truth, so its query string replaces the params.
    setUrl(url: string) {
      this.draft.url = url
      const qi = url.indexOf('?')
      if (qi === -1) {
        this.draft.params = []
        return
      }
      try {
        const sp = new URLSearchParams(url.slice(qi + 1))
        const next: KeyValueRow[] = []
        sp.forEach((value, name) => next.push({ name, value, enabled: true }))
        this.draft.params = next
      } catch {
        // Invalid query string — leave params untouched.
      }
    },
    // The params → URL direction: rebuilds the query string from enabled rows only, preserving
    // order, without normalizing the rest of the URL as the URL API would.
    syncParamsToUrl() {
      const base = this.draft.url.split('?')[0]
      const sp = new URLSearchParams()
      for (const p of this.draft.params) {
        if (p.enabled && p.name.trim()) sp.append(p.name.trim(), p.value)
      }
      // URLSearchParams percent-encodes braces, so `{{tenant}}` would come back as
      // `%7B%7Btenant%7D%7D` and stop being a token; restore just those delimiters.
      const qs = sp.toString().replace(/%7B%7B/g, '{{').replace(/%7D%7D/g, '}}')
      this.draft.url = qs ? `${base}?${qs}` : base
    },
    addParam() {
      this.draft.params.push({ name: '', value: '', enabled: true })
    },
    removeParam(i: number) {
      this.draft.params.splice(i, 1)
      this.syncParamsToUrl()
    },
    toggleParam(i: number) {
      this.draft.params[i].enabled = !this.draft.params[i].enabled
      this.syncParamsToUrl()
    },
    updateParam(i: number, patch: Partial<KeyValueRow>) {
      this.draft.params[i] = { ...this.draft.params[i], ...patch }
      this.syncParamsToUrl()
    },
    addHeader() {
      this.draft.headers.push({ name: '', value: '', enabled: true })
    },
    removeHeader(i: number) {
      this.draft.headers.splice(i, 1)
    },
    toggleHeader(i: number) {
      this.draft.headers[i].enabled = !this.draft.headers[i].enabled
    },
    updateHeader(i: number, patch: Partial<KeyValueRow>) {
      this.draft.headers[i] = { ...this.draft.headers[i], ...patch }
    },
    clear() {
      this.requests = []
      this.manualId = null
      this.browserId = null
      this.unreadCount = 0
    },
    clearRequests(ids: string[]) {
      if (ids.length === 0) return
      const set = new Set(ids)
      this.requests = this.requests.filter((r) => !set.has(r.id))
      if (this.manualId && set.has(this.manualId)) this.manualId = null
      if (this.browserId && set.has(this.browserId)) this.browserId = null
    },
  },
})
