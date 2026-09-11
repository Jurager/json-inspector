import { defineStore } from 'pinia'
import type { RequestRecord } from '../lib/types'

let counter = 0
function nextId(): string {
  counter += 1
  return `req-${Date.now()}-${counter}`
}

// Recording flips back to false after 30s of silence from the extension —
// module-level like `counter` above, since it's bookkeeping for the store's
// own action rather than reactive UI state itself.
let recordingTimer: ReturnType<typeof setTimeout> | null = null
const RECORDING_TIMEOUT_MS = 30_000

// Capture is the status bar's source of truth for the browser extension's
// state. Real signals arrive on step 9; until then the only event is the
// arrival of a captured request.
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

// The in-progress request being assembled in the command line. Lived in
// RequestBuilder's local refs before the redesign; it moved into the store so
// "Открыть в «Запросе»" (step 5) can fill it from a captured record.
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
    // The manual request shown in the "Запрос" view.
    manualId: null as string | null,
    // The captured request selected in the "Браузер" view.
    browserId: null as string | null,
    activeView: 'request' as 'request' | 'browser',
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
    // Incremented by "Открыть в «Запросе»" to nudge RequestBuilder to focus its
    // URL field after it remounts.
    focusUrlTick: 0,
  }),
  getters: {
    manualSelected(state): RequestRecord | null {
      return state.requests.find((r) => r.id === state.manualId) ?? null
    },
    browserSelected(state): RequestRecord | null {
      return state.requests.find((r) => r.id === state.browserId) ?? null
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
    },
    selectBrowser(id: string) {
      this.browserId = id
    },
    markBrowserRead() {
      this.unreadCount = 0
    },
    setCaptureState(partial: Partial<CaptureState>) {
      this.capture = { ...this.capture, ...partial }
      // While recording, keep the 30s watchdog alive: each captured request
      // pushes the deadline back, so `recording` only drops after a real
      // silence, not between two requests a second apart.
      if (this.capture.recording) {
        if (recordingTimer) clearTimeout(recordingTimer)
        recordingTimer = setTimeout(() => {
          this.capture = { ...this.capture, recording: false }
        }, RECORDING_TIMEOUT_MS)
      } else if (recordingTimer) {
        clearTimeout(recordingTimer)
        recordingTimer = null
      }
    },
    setOpenChip(chip: 'params' | 'headers' | 'auth' | 'body' | null) {
      this.openChip = chip
    },
    requestFocusUrl() {
      this.focusUrlTick++
    },
    // Fills the draft from a captured record without sending it — the
    // "Открыть в «Запросе»" action turns a read-only captured request back into
    // an editable draft.
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
    // setUrl is the URL → params direction of the two-way sync: a manual URL
    // edit is the source of truth, so its query string replaces the params.
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
    // syncParamsToUrl is the params → URL direction: rebuilds the query string
    // from enabled rows only, preserving order, without normalizing the rest of
    // the URL the way the URL API would.
    syncParamsToUrl() {
      const base = this.draft.url.split('?')[0]
      const sp = new URLSearchParams()
      for (const p of this.draft.params) {
        if (p.enabled && p.name.trim()) sp.append(p.name.trim(), p.value)
      }
      const qs = sp.toString()
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
