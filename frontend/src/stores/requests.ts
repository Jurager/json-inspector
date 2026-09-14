import { defineStore } from 'pinia'
import { t as tr } from '../i18n'
import { bodyMissing, recordView, type RecordBodies, type RecordView } from '../lib/requestRecord'
import {
  BodyKind,
  BodySide,
  DraftID,
  RecordSource,
  RowKind,
  type Auth,
  type CookieRow,
  type Draft,
  type FormRow,
  type Record,
  type Row,
  type Scripts,
} from '../../bindings/json-inspector/internal/domain'
import {
  TextField,
  type Preview,
  type RowPatch,
  type Seed,
  type State,
  type TextResult,
} from '../../bindings/json-inspector/internal/usecase/draft'
import type { Level } from '../../bindings/json-inspector/internal/usecase/scripting'
import type { ChipName } from '../lib/requestSource'
import { DraftService, RecordsService, ScriptingService } from '../../bindings/json-inspector/internal/transport/wails'
import { useEnvironmentsStore } from './environments'
import { focusUrlField } from '../composables/urlFocus'

// The draft this store edits: the one the command line composes. A collection card has a draft of
// its own, keyed by the node it came from, and it is the collections store that holds it.
const DRAFT = DraftID.DraftCommandLine

// What the history lived in before it moved into the database.
const LEGACY_KEY = 'ji-history-v1'

// How long the window holds a text it is typing before handing it over. The draft is Go's, so every
// keystroke is a write to the database on the other side; a pause is what keeps that from being one
// write per character. Anything that ends the moment — blur, Enter, sending — flushes at once.
const FLUSH_MS = 400

interface CaptureState {
  connected: boolean
  recording: boolean
  paused: boolean
  tabs: number
}

type AuthType = Auth['type']

// The mirror: every record and the draft itself come from Go. What the window owns is the two texts
// it is typing into — the command line and the body — and where the caret is in them.
export const useRequestsStore = defineStore('requests', {
  state: () => ({
    // ---- history ---------------------------------------------------------
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
    // The attempts this window started. An answer is adopted only if it answers one of them: a
    // collection run sends through the same path, and its records belong in history, not in the
    // command line's pane.
    mine: [] as string[],
    capture: { connected: false, recording: false, paused: false, tabs: 0 } as CaptureState,

    // ---- the draft -------------------------------------------------------
    draft: null as Draft | null,
    preview: { missing: [], canSend: false } as Preview,
    // The two texts the window is typing. Rev counts this window's own flushes, and it comes back
    // with every answer: a reply to a keystroke that has since been typed over is recognised by it.
    urlText: '',
    bodyText: '',
    urlRev: 0,
    bodyRev: 0,
    // Whether what the window holds is newer than what Go has. Until its own flush comes back, the
    // text under the caret is the only truth about it.
    bufferedUrl: false,
    bufferedBody: false,
    // The pending hand-over of what is being typed. Not state the window draws: it is the clock the
    // flush runs on.
    flushTimer: null as ReturnType<typeof setTimeout> | null,

    openChip: null as ChipName | null,

    // The code the command line runs around its own request — nil when it has none — and the chain it
    // stands in. Nothing is above a request composed here, so the chain is the draft's own level or
    // nothing at all: the box says «выполняется», not «наследуется».
    scriptsFor: null as string | null,
    scripts: null as Scripts | null,
    chain: [] as Level[],
    focusTabId: null as number | null,
    inspector: { open: false, path: null as string | null, width: 300 },
  }),
  getters: {
    // The draft, read the way the command line reads it: a text that is being typed is the window's,
    // everything else is Go's.
    url(state): string {
      return state.urlText
    },
    body(state): string {
      return state.bodyText
    },
    method(state): string {
      return state.draft?.method ?? 'GET'
    },
    params(state): Row[] {
      return state.draft?.params ?? []
    },
    headers(state): Row[] {
      return state.draft?.headers ?? []
    },
    cookies(state): CookieRow[] {
      return state.draft?.cookies ?? []
    },
    auth(state): Auth {
      return state.draft?.auth ?? { type: 'none' as AuthType, token: '' }
    },
    // The command line composes one request and nothing stands above it, so its Auth chip offers the
    // four kinds and no «Наследовать».
    canInherit(): boolean {
      return false
    },
    inheritedAuth(): Auth | null {
      return null
    },
    missingVars(state): string[] {
      // The wire marks a list as possibly null because Go can marshal a nil slice that way.
      return state.preview.missing ?? []
    },
    enabledParamsCount(state): number {
      return (state.draft?.params ?? []).filter((p) => p.enabled && p.name.trim()).length
    },
    enabledHeadersCount(state): number {
      return (state.draft?.headers ?? []).filter((h) => h.enabled && h.name.trim()).length
    },
    // The three the body is made of. Only the text is the window's while it is being typed; the
    // format, the form rows and the file path come from Go like every other row of the request.
    bodyKind(state): BodyKind {
      return state.draft?.bodyKind ?? BodyKind.BodyRaw
    },
    form(state): FormRow[] {
      return state.draft?.form ?? []
    },
    bodyFile(state): string {
      return state.draft?.bodyFile ?? ''
    },
    // What the dashed chip goes solid on. The same rule in both stores, so the chip cannot disagree
    // with itself depending on which one is drawing it.
    hasBody(state): boolean {
      const draft = state.draft
      if (!draft) return false
      switch (draft.bodyKind) {
        case BodyKind.BodyForm:
          return (draft.form ?? []).some((row) => row.enabled && row.name.trim())
        case BodyKind.BodyBinary:
          return (draft.bodyFile ?? '') !== ''
        default:
          return state.bodyText.trim().length > 0
      }
    },

    manualSelected(state): RecordView | null {
      return viewOf(state, state.manualId)
    },
    browserSelected(state): RecordView | null {
      return viewOf(state, state.browserId)
    },
    // The level the code belongs to here is always the same one: the command line's draft.
    scriptsLevel(): string {
      return DRAFT
    },
    // What the level would run for one half if its own box stayed empty. Nothing is above the command
    // line, so this is the empty answer — kept so both request sources answer the same way.
    inheritedScript(): (_scope: 'pre' | 'post') => { text: string; name: string } | null {
      return () => null
    },
  },

  actions: {
    // ---- the draft, from Go ----------------------------------------------

    async loadDraft() {
      this.apply(await DraftService.Snapshot(DRAFT))
    },

    // The code of the request being composed: written into the draft's own row, and read back from it.
    async loadScripts() {
      this.scripts = await ScriptingService.Scripts(DRAFT)
      this.chain = (await ScriptingService.Chain(DRAFT)) ?? []
    },

    async saveScripts(pre: string, post: string) {
      const written = pre.trim() || post.trim() ? { pre, post } : null
      const saved = await ScriptingService.SaveScripts(DRAFT, written)
      const chain = (await ScriptingService.Chain(DRAFT)) ?? []
      this.scriptsFor = DRAFT
      this.scripts = saved
      this.chain = chain
    },

    // Every answer from the draft side lands here: the draft and its preview are replaced, and the
    // two texts only when the window is not holding something newer.
    apply(state: State) {
      this.draft = state.draft
      this.preview = state.preview
      if (!this.bufferedUrl) this.urlText = state.draft.url
      if (!this.bufferedBody) this.bodyText = state.draft.body
    },

    // The command line is what the user is typing, so it is the window's until the flush of that
    // very text comes back. A later answer for an earlier revision is dropped rather than applied:
    // by then it describes a text nobody is looking at.
    applyText(result: TextResult) {
      if (result.field === TextField.FieldURL) {
        if (result.rev !== this.urlRev) return
        this.bufferedUrl = false
      } else {
        if (result.rev !== this.bodyRev) return
        this.bufferedBody = false
      }
      this.apply(result)
    },

    setUrl(text: string) {
      this.urlText = text
      this.urlRev += 1
      this.bufferedUrl = true
      this.scheduleFlush()
    },

    setBody(text: string) {
      this.bodyText = text
      this.bodyRev += 1
      this.bufferedBody = true
      this.scheduleFlush()
    },

    scheduleFlush() {
      if (this.flushTimer) clearTimeout(this.flushTimer)
      this.flushTimer = setTimeout(() => void this.flush(), FLUSH_MS)
    },

    // Hand over whatever the window is still holding. Blur, Enter, sending and a hidden window all
    // call this, so nothing typed is ever left behind on this side.
    async flush() {
      if (this.flushTimer) {
        clearTimeout(this.flushTimer)
        this.flushTimer = null
      }
      if (this.bufferedUrl) {
        const rev = this.urlRev
        this.applyText(await DraftService.SetText(DRAFT, { field: TextField.FieldURL, text: this.urlText, rev }))
      }
      if (this.bufferedBody) {
        const rev = this.bodyRev
        this.applyText(await DraftService.SetText(DRAFT, { field: TextField.FieldBody, text: this.bodyText, rev }))
      }
    },

    async setMethod(method: string) {
      this.apply(await DraftService.SetMethod(DRAFT, method))
    },

    async setAuth(auth: Auth) {
      this.apply(await DraftService.SetAuth(DRAFT, auth))
    },

    async setBodyKind(kind: BodyKind) {
      this.apply(await DraftService.SetBodyKind(DRAFT, kind))
    },

    async setBodyFile(path: string) {
      this.apply(await DraftService.SetBodyFile(DRAFT, path))
    },

    // The file is Go's to open and Go's to read: the dialog is native, and the window never holds
    // the bytes. A closed dialog answers with nothing, which is not a failure.
    async pickBodyFile(): Promise<string> {
      return await DraftService.PickBodyFile(tr('files.bodyFile'), tr('files.allFiles'))
    },

    async addRow(kind: RowKind): Promise<string> {
      const before = this.idsOf(kind)
      this.apply(await DraftService.AddRow(DRAFT, kind))
      const after = this.idsOf(kind)
      return after.find((id) => !before.includes(id)) ?? ''
    },

    async removeRow(kind: RowKind, id: string) {
      this.apply(await DraftService.RemoveRow(DRAFT, kind, id))
    },

    async patchRow(kind: RowKind, id: string, patch: RowPatch) {
      this.apply(await DraftService.PatchRow(DRAFT, kind, id, patch))
    },

    async toggleRow(kind: RowKind, id: string, enabled: boolean) {
      await this.patchRow(kind, id, { enabled })
    },

    // A whole request handed to the draft: a record opened in the command line, or a command
    // pasted into it.
    async replace(seed: Seed) {
      // A flush of the text that is being replaced must not land on the draft that replaced it.
      if (this.flushTimer) {
        clearTimeout(this.flushTimer)
        this.flushTimer = null
      }
      this.urlRev += 1
      this.bodyRev += 1
      this.bufferedUrl = false
      this.bufferedBody = false
      this.apply(await DraftService.Replace(DRAFT, seed))
    },

    idsOf(kind: RowKind): string[] {
      if (kind === RowKind.RowParams) return this.params.map((r) => r.id)
      if (kind === RowKind.RowHeaders) return this.headers.map((r) => r.id)
      return this.cookies.map((r) => r.id ?? '')
    },

    // ---- history, from Go ------------------------------------------------

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
      const bodies = await this.loadBodies(id)
      await this.replace(recordSeed(record, bodies.request ?? record.requestBody?.inline ?? ''))
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

    // What is composed goes out through Go, which is the side that can fill the variables in: the
    // window sends nothing but the order to send. The flush first is what makes the request the one
    // the user is looking at.
    async send() {
      await this.flush()
      this.loading = true
      const id = await RecordsService.Send(DRAFT)
      this.claim(id)
      // The record can outrun the call that asked for it: if it has already arrived, the spinner is
      // off and putting the id back would leave the button waiting for what it just got.
      if (this.loading) this.pendingId = id
    },

    // The attempt becomes this pane's: the answer to it will be adopted, and an answer to anything
    // else — a run's request — will not.
    claim(id: string) {
      this.mine = [id, ...this.mine].slice(0, 20)
    },

    // A request that is not the draft: following a link out of a response must not disturb what is
    // being composed.
    async sendSpec(seed: Seed) {
      this.loading = true
      const id = await RecordsService.SendSpec(seed)
      this.claim(id)
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

    // ---- the window's own state ------------------------------------------

    setOpenChip(chip: ChipName | null) {
      this.openChip = chip
    },

    focusSearch() {
      this.activeView = 'request'
      focusUrlField()
    },

    setInspector(partial: Partial<{ open: boolean; path: string | null; width: number }>) {
      this.inspector = { ...this.inspector, ...partial }
    },
  },
})

// A record as the draft takes it: what "открыть в запросе" hands over. The jar comes from the
// record's own rows, and the Cookie header is left beside them for the side that decides which of
// the two goes out.
export function recordSeed(record: Record, requestBody: string): Seed {
  return {
    method: record.method,
    url: record.url,
    headers: record.requestHeaders ?? [],
    body: requestBody,
    cookies: record.requestCookies ?? [],
  }
}

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

