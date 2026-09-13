import { defineStore } from 'pinia'
import type { CookieRow } from '../lib/cookies'
import { cookieRowFrom, emptyCookieRow, parseCookieHeader } from '../lib/cookies'
import { bodyMissing, recordView, type RecordBodies, type RecordView } from '../lib/requestRecord'
import type { ParsedRequest } from '../lib/parseRequest'
import {
  BodySide,
  RecordSource,
  type CookieRow as WireCookieRow,
  type HeaderPair,
  type Record,
} from '../../bindings/json-inspector/internal/domain'
import type { SendInput } from '../../bindings/json-inspector/internal/usecase/record'
import { RecordsService } from '../../bindings/json-inspector/internal/transport/wails'
import { useEnvironmentsStore } from './environments'
import { focusUrlField } from '../composables/urlFocus'

// What the history lived in before it moved into the database.
const LEGACY_KEY = 'ji-history-v1'

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

interface DraftState {
  method: string
  url: string
  params: KeyValueRow[]
  headers: KeyValueRow[]
  auth: { type: AuthType; token: string }
  body: string
  cookies: CookieRow[]
}

// The mirror: every record comes from Go, and the list holds what Go listed. The one thing kept
// beside it is the body text a record did not travel with — read once per record, when it is opened.
export const useRequestsStore = defineStore('requests', {
  state: () => ({
    records: [] as Record[],
    // An index signature rather than Record<>: the domain type of the same name would win.
    bodies: {} as { [id: string]: RecordBodies },
    manualId: null as string | null,
    browserId: null as string | null,
    activeView: 'request' as 'request' | 'browser' | 'collections',
    unreadCount: 0,
    loading: false,
    // The attempt the spinner belongs to. A request outlives the call that started it, so this is
    // the only handle on it until its record arrives.
    pendingId: null as string | null,
    capture: { connected: false, recording: false, tabs: 0 } as CaptureState,
    draft: {
      method: 'GET',
      url: '',
      params: [] as KeyValueRow[],
      headers: [{ name: 'Accept', value: 'application/vnd.api+json', enabled: true }] as KeyValueRow[],
      auth: { type: 'none' as AuthType, token: '' },
      body: '',
      cookies: [] as CookieRow[],
    } as DraftState,
    openChip: null as 'params' | 'headers' | 'auth' | 'body' | null,
    focusTabId: null as number | null,
    inspector: { open: false, path: null as string | null, width: 300 },
  }),
  getters: {
    manualSelected(state): RecordView | null {
      return viewOf(state, state.manualId)
    },
    browserSelected(state): RecordView | null {
      return viewOf(state, state.browserId)
    },
    missingVars(state): string[] {
      const envs = useEnvironmentsStore()
      const parts = [
        state.draft.url,
        ...state.draft.params.filter((p) => p.enabled).map((p) => `${p.name}\n${p.value}`),
        ...state.draft.headers.filter((h) => h.enabled).map((h) => `${h.name}\n${h.value}`),
        state.draft.body,
        state.draft.auth.token,
        ...state.draft.cookies.map((c) => `${c.name}\n${c.value}`),
      ]
      return envs.missingVarNames(parts.join('\n'))
    },
  },
  actions: {
    // ---- history, from Go -------------------------------------------------

    async load() {
      this.records = (await RecordsService.List(RecordSource.$zero, 0)) ?? []
    },

    // The old build kept history in localStorage; it moves into the database on the first launch of
    // this one. The raw string goes over as it is — reading that shape is Go's job — and the key is
    // dropped only once the import is through, so a failure is retried on the next launch.
    async importLegacyOnce() {
      const raw = localStorage.getItem(LEGACY_KEY)
      if (raw === null) return
      try {
        await RecordsService.ImportLegacy(raw)
        localStorage.removeItem(LEGACY_KEY)
      } catch {
        // The key stays: the next launch tries again.
      }
    },

    // A record arriving from Go — one this app sent, the browser did, or the sample is. It lands at
    // the top, and a record already in the list is replaced rather than added twice.
    prepend(record: Record) {
      this.records = [record, ...this.records.filter((r) => r.id !== record.id)]
    },

    async addSent(record: Record) {
      this.prepend(record)
      this.manualId = record.id
      await this.loadBodies(record.id)
    },

    addIngested(record: Record) {
      this.prepend(record)
      // The badge belongs to the browser rail, so only a capture counts towards it.
      if (record.source === RecordSource.SourceBrowser && this.activeView !== 'browser') {
        this.unreadCount += 1
      }
    },

    async selectManual(id: string) {
      const record = this.records.find((r) => r.id === id)
      if (!record) return
      this.manualId = id
      this.loadDraft(recordView(record, await this.loadBodies(id)))
    },

    async selectBrowser(id: string) {
      this.browserId = id
      await this.loadBodies(id)
    },

    clearUnreadCaptures() {
      this.unreadCount = 0
    },

    focusBrowserTab(tabId: number) {
      this.activeView = 'browser'
      this.unreadCount = 0
      this.focusTabId = tabId
    },

    setCaptureState(partial: Partial<CaptureState>) {
      this.capture = { ...this.capture, ...partial }
    },

    // A body the record did not bring with it is read once, by name, and kept for as long as the
    // window is open. A small one came with the record, so this costs nothing in the common case.
    async loadBodies(id: string): Promise<RecordBodies> {
      const cached = this.bodies[id]
      if (cached) return cached

      const record = this.records.find((r) => r.id === id)
      if (!record) return {}

      const bodies: RecordBodies = {
        request: record.requestBody?.inline,
        response: record.responseBody?.inline,
      }
      if (bodyMissing(record.requestBody)) {
        bodies.request = await RecordsService.Body(id, BodySide.SideRequest)
      }
      if (bodyMissing(record.responseBody)) {
        bodies.response = await RecordsService.Body(id, BodySide.SideResponse)
      }
      this.bodies[id] = bodies
      return bodies
    },

    // ---- sending ----------------------------------------------------------

    // The request is ready to go and its masked copy is built beside it; what comes of it arrives
    // as an event. Only the id comes back, which is exactly what cancelling needs.
    async send(input: SendInput) {
      this.loading = true
      const id = await RecordsService.Send(input)
      // The record can outrun the call that asked for it: if it has already arrived, the spinner is
      // off and putting the id back would leave the button waiting for what it just got.
      if (this.loading) this.pendingId = id
    },

    async cancel() {
      const id = this.pendingId
      this.pendingId = null
      this.loading = false
      if (id) await RecordsService.Cancel(id)
    },

    async finishSend(record: Record) {
      this.pendingId = null
      this.loading = false
      await this.addSent(record)
    },

    failSend() {
      this.pendingId = null
      this.loading = false
    },

    async clearRecords(ids: string[]) {
      if (ids.length === 0) return
      await RecordsService.Clear(ids)

      const gone = new Set(ids)
      this.records = this.records.filter((r) => !gone.has(r.id))
      for (const id of ids) delete this.bodies[id]
      if (this.manualId && gone.has(this.manualId)) this.manualId = null
      if (this.browserId && gone.has(this.browserId)) this.browserId = null
    },

    // ---- the draft --------------------------------------------------------

    setOpenChip(chip: 'params' | 'headers' | 'auth' | 'body' | null) {
      this.openChip = chip
    },

    focusSearch() {
      this.activeView = 'request'
      focusUrlField()
    },

    setInspector(partial: Partial<{ open: boolean; path: string | null; width: number }>) {
      this.inspector = { ...this.inspector, ...partial }
    },

    // The draft takes over a request — one out of history, or one just pasted into the URL field.
    // Both are a method, a URL, headers and a body; only the source of the headers differs.
    adopt(request: { method: string; url: string; headers: HeaderPair[]; body: string; cookies: WireCookieRow[] }) {
      this.draft.method = request.method
      this.draft.auth = { type: 'none', token: '' }
      this.setUrl(request.url)

      // The Cookie header is the jar's own projection, so it never also stays a header row.
      const rawCookieHeader = request.headers.find((h) => h.name.toLowerCase() === 'cookie')?.value
      this.draft.headers = request.headers
        .filter((h) => h.name.toLowerCase() !== 'cookie')
        .map((h) => ({ name: h.name, value: h.value, enabled: true }))
      this.draft.body = request.body

      // A record keeps the jar its draft was edited with; a pasted command has only the header,
      // and that is where its cookies are recovered from.
      this.draft.cookies = request.cookies.length
        ? request.cookies.map(cookieRowFrom)
        : rawCookieHeader
          ? parseCookieHeader(rawCookieHeader)
          : []
    },

    loadDraft(record: RecordView) {
      this.adopt({
        method: record.method,
        url: record.url,
        headers: record.requestHeaders,
        body: record.requestBody,
        cookies: record.requestCookies,
      })
    },

    // A command carries its headers as a map, because that is what a command has.
    loadCommand(parsed: ParsedRequest) {
      this.adopt({
        method: parsed.method,
        url: parsed.url,
        headers: Object.entries(parsed.requestHeaders).map(([name, value]) => ({ name, value })),
        body: parsed.requestBody,
        cookies: [],
      })
    },

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

    syncParamsToUrl() {
      const base = this.draft.url.split('?')[0]
      const sp = new URLSearchParams()
      for (const p of this.draft.params) {
        if (p.enabled && p.name.trim()) sp.append(p.name.trim(), p.value)
      }
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

    addCookie() {
      this.draft.cookies.push(emptyCookieRow())
    },

    removeCookie(i: number) {
      this.draft.cookies.splice(i, 1)
    },

    updateCookie(i: number, patch: Partial<CookieRow>) {
      this.draft.cookies[i] = { ...this.draft.cookies[i], ...patch }
    },

    toggleCookieFlag(i: number, flag: 'secure' | 'httpOnly') {
      this.draft.cookies[i] = { ...this.draft.cookies[i], [flag]: !this.draft.cookies[i][flag] }
    },
  },
})

// A record as the pane draws it, or nothing when the id names nothing. The lookup lives out here
// because two getters do the same thing and a getter cannot call another one cleanly.
function viewOf(
  state: { records: Record[]; bodies: { [id: string]: RecordBodies } },
  id: string | null
): RecordView | null {
  if (!id) return null
  const record = state.records.find((r) => r.id === id)
  return record ? recordView(record, state.bodies[id]) : null
}
