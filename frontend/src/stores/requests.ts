import { defineStore } from 'pinia'
import { t as tr } from '../i18n'
import { bodyMissing, recordView, type RecordBodies, type RecordView } from '../lib/requestRecord'
import { takeAnswer } from '../lib/earlyAnswers'
import {
  BodyKind,
  BodySide,
  DraftID,
  RecordSource,
  RowKind,
  type Auth,
  type AuthToken,
  type CookieRow,
  type Draft,
  type FormRow,
  ProjectedRow,
  type Record,
  type Row,
  type Scripts,
} from '../../bindings/json-inspector/internal/domain'
import {
  TextField,
  type CommandResult,
  type Preview,
  type RowPatch,
  type Seed,
  type State,
  type TextResult,
} from '../../bindings/json-inspector/internal/usecase/draft'
import type { Level } from '../../bindings/json-inspector/internal/usecase/scripting'
import { NO_AUTH } from '../lib/requestSource'
import type { ChipName } from '../lib/requestSource'
import {
  BridgeService,
  DraftService,
  RecordsService,
  ScriptingService,
} from '../../bindings/json-inspector/internal/transport/wails'
import { useSettings } from '../composables/useSettings'
import { useEnvironmentsStore } from './environments'
import { useWorkspacesStore } from './workspaces'
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

// What one tab under capture is: which tab, and since when. The time is unix milliseconds and comes
// from the extension, which is the only side that knows when it armed the tab.
export interface CaptureTab {
  tabId: number
  since: number
}

interface CaptureState {
  connected: boolean
  recording: boolean
  paused: boolean
  tabs: number
  // What the browser calls itself, version included: the extension reads it from its own user agent
  // and the page draws it beside the name.
  browser: string
  tabList: CaptureTab[]
}

type AuthType = Auth['type']

// The mirror: every record and the draft itself come from Go. What the window owns is the two texts
// it is typing into — the command line and the body — and where the caret is in them.
export const useRequestsStore = defineStore('requests', {
  state: () => ({
    // ---- history ---------------------------------------------------------
    // The history of one workspace, and which one it is a history of. The list is per space, so a row
    // that says it belongs somewhere else is a row the window has switched away from.
    workspaceId: '' as string,
    records: [] as Record[],
    // An index signature rather than Record<>: the domain type of the same name would win.
    bodies: {} as { [id: string]: RecordBodies },
    manualId: null as string | null,
    browserId: null as string | null,
    // The tab of captured traffic the window is looking at. A record and a tab are two answers to
    // "what is on screen" and only one of them is drawn: selecting either clears the other.
    browserTabKey: null as string | null,
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
    capture: {
      connected: false,
      recording: false,
      paused: false,
      tabs: 0,
      browser: '',
      tabList: [],
    } as CaptureState,

    // ---- the draft -------------------------------------------------------
    draft: null as Draft | null,
    // The draft's own revision as it was when the line was last filled — a record opened, a command
    // pasted, the workspace's draft as this session found it. Every edit Go counts moves the draft's
    // revision on, so comparing the two is what says whether the person has touched the line; see
    // lineDirty.
    lineRevision: 0,
    // A record waiting for an answer: opening one replaces what the line is composing, which is not
    // done quietly, and the click waits here until the window says what to do with the work.
    pendingOpen: null as null | { resolve: (ok: boolean) => void },
    // What the draft's authorization comes to, as Go worked it out: the rows it puts in the parameter
    // and header lists, and the state of a token somebody else issued. Neither is part of the draft.
    projected: [] as ProjectedRow[],
    token: null as AuthToken | null,
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
    inspector: { open: false, path: null as string | null, width: 330 },
  }),
  getters: {
    // Whether a record belongs in the list being drawn. The extension keeps capturing while the user
    // switches, and an attempt sent from one space can come back after the window has moved to
    // another — reading a record is by id, and an id says nothing about where it belongs. Until both
    // sides know the workspace (the first frames, before anything is loaded) a row is taken as it
    // comes: the list itself came from Go already scoped.
    accepts(state) {
      return (record: Record) =>
        !state.workspaceId || !record.workspaceId || record.workspaceId === state.workspaceId
    },

    // Whether the line holds work of its own: a request somebody typed or edited here, which a
    // record opened over it would throw away.
    //
    // It is the draft's revision against the one the line was filled at, and not a look at what is
    // in the line. Opening a record and then another one throws nothing away — the first is a
    // record, it is in the history, and the line was holding a copy of it — so the two look
    // different while neither is work. What is work is an edit, Go counts every one of them in
    // Draft.Revision, and the count only moves when somebody touches the line.
    //
    // The address is asked for as well: a line with nothing in it is not a request, whatever else
    // was changed about it.
    lineDirty(state): boolean {
      if (!state.draft || !state.draft.url.trim()) return false
      return state.draft.revision !== state.lineRevision
    },

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
      return state.draft?.auth ?? NO_AUTH
    },
    // `projected` and `token` are fields of this store rather than getters — Go sends them with every
    // answer, so there is nothing to derive.
    //
    // The command line composes one request and nothing stands above it, so its Auth chip offers the
    // schemes a request can have and no «Наследовать».
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

    // The preview — which `{{tokens}}` resolve to nothing — is worked out on the draft side, and a
    // variable added in the environments window is not an edit of this line: without asking again,
    // the block that says the send is held would outlive the variable that caused it. Only the
    // preview is taken, so nothing the window is holding in its buffers is touched.
    async refreshPreview() {
      this.preview = (await DraftService.Snapshot(DRAFT)).preview
    },

    async loadDraft() {
      this.apply(await DraftService.Snapshot(DRAFT))
      // The line is where this workspace left it. Whatever was typed here yesterday is not an edit
      // somebody made now, and asking about it is asking the wrong question.
      this.lineRevision = this.draft?.revision ?? 0
    },

    // The code of the request being composed: written into the draft's own row, and read back from it.
    async loadScripts() {
      this.scripts = await ScriptingService.Scripts(DRAFT)
      this.chain = (await ScriptingService.Chain(DRAFT)) ?? []
    },

    async saveScripts(pre: string, post: string, off: { pre: boolean; post: boolean }) {
      const flags = {
        preOff: off.pre && !!pre.trim(),
        postOff: off.post && !!post.trim(),
      }
      const written = pre.trim() || post.trim() ? { pre, post, ...flags } : null
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
      this.projected = state.projected ?? []
      this.token = state.token ?? null
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

    // A row the authorization put in a list is not stored, so an edit to it is an edit to the field
    // behind it and a deletion is the request no longer authorizing itself.
    async patchDerived(target: RowKind, name: string, value: string) {
      this.apply(await DraftService.PatchDerived(DRAFT, target, name, value))
    },

    async removeDerived() {
      this.apply(await DraftService.RemoveDerived(DRAFT))
    },

    // «Получить токен» and «Очистить». A failure is not swallowed: the user asked in so many words,
    // and the refusal is the answer they are waiting for.
    async obtainAuth() {
      this.apply(await DraftService.ObtainAuth(DRAFT))
    },

    async forgetAuth() {
      this.apply(await DraftService.ForgetAuth(DRAFT))
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
      // Filled, not edited: what stands in the line is that request and nothing of anybody's own.
      this.lineRevision = this.draft?.revision ?? 0
    },

    // A command pasted into the line. Reading it and handing it to the draft is one call on the
    // other side — carrying the request across the window only to send it back would be this side
    // taking apart what Go had just built. What stays here is the buffering: which text the window
    // is holding is the window's business, and a flush of the text being replaced must not land on
    // the draft that replaced it.
    //
    // The reading comes back either way. A paste that was not a command returns no state at all,
    // and the window puts the text in the field itself.
    async pasteCommand(text: string): Promise<CommandResult> {
      if (this.flushTimer) {
        clearTimeout(this.flushTimer)
        this.flushTimer = null
      }
      this.urlRev += 1
      this.bodyRev += 1
      this.bufferedUrl = false
      this.bufferedBody = false

      const pasted = await DraftService.PasteCommand(DRAFT, text)
      if (pasted.state) {
        this.apply(pasted.state)
        // A command is a request handed over whole, like a record: the line holds it and nothing of
        // anybody's own.
        this.lineRevision = this.draft?.revision ?? 0
      }
      return pasted.reading
    },

    idsOf(kind: RowKind): string[] {
      if (kind === RowKind.RowParams) return this.params.map((r) => r.id)
      if (kind === RowKind.RowHeaders) return this.headers.map((r) => r.id)
      return this.cookies.map((r) => r.id ?? '')
    },

    // ---- history, from Go ------------------------------------------------

    async load() {
      // Which workspace this list is about is asked from the store that owns the pointer, so the two
      // cannot disagree: a switch reloads the list and renames what it is a list of, in one call.
      this.workspaceId = useWorkspacesStore().activeId
      this.records = (await RecordsService.List(RecordSource.$zero, 0)) ?? []
    },

    // What a switch leaves behind: nothing on screen is about the workspace being entered, and a body
    // read for a record of the old one must not be handed to a record of the new one.
    forget() {
      this.records = []
      this.bodies = {}
      this.manualId = null
      this.browserId = null
      this.unreadCount = 0
      this.mine = []
      this.pendingId = null
      this.loading = false
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
      if (!this.accepts(record)) return
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

    // A record picked out of the history — a click on a row, the palette landing on it, the arrow
    // that walks the list. It becomes the request being composed, the jar it was sent with and all,
    // and the pane goes on showing it: in this rail a record *is* a request that was made, so there
    // is nothing else for the line to hold, and a row lit while the line holds something else would
    // name a request it has nothing to do with.
    //
    // It replaces what was being typed, which is why the window is asked first when the line holds
    // work of its own — see lineDirty and the alert that answers it.
    async selectManual(id: string) {
      const record = this.records.find((r) => r.id === id)
      if (!record) return
      if (this.lineDirty && !(await this.confirmOpen())) return

      const bodies = await this.loadBodies(id)
      this.manualId = id
      await this.replace(recordSeed(record, bodies.request ?? record.requestBody?.inline ?? ''))
    },

    // The click waits here until the window answers. Nothing is dropped before that: what the
    // answer decides is whether the record is opened at all, so the pending call is held and not
    // acted on — a record opened over unsaved work is not a thing to do and undo.
    confirmOpen(): Promise<boolean> {
      return new Promise((resolve) => {
        this.pendingOpen = { resolve }
      })
    },

    answerUnsaved(action: 'discard' | 'cancel') {
      const pending = this.pendingOpen
      if (!pending) return
      this.pendingOpen = null
      pending.resolve(action === 'discard')
    },

    // A capture is a different thing: the browser rail has no command line, so there a record is
    // only shown. «Открыть в „Запросе"» on one is what carries it across — see ResponseViewer.
    // Looking at a tab rather than at one of its requests. The pane it opens is the tab's own page,
    // and the key is what that page is about — the same key the list groups its records by.
    selectBrowserTab(key: string) {
      this.browserTabKey = key
      this.browserId = null
    },

    async selectBrowser(id: string) {
      this.browserId = id
      // One thing on screen at a time: a request inside a tab replaces the tab.
      this.browserTabKey = null
      await this.loadBodies(id)
    },

    clearUnreadCaptures() {
      this.unreadCount = 0
    },

    // The command line stops being a record it was showing. What it holds now is a request being
    // composed, and a row left lit in the history would name a record it has nothing to do with —
    // and the pane would go on drawing that record's response beside it.
    deselectManual() {
      this.manualId = null
    },

    // The extension asking the window to look at a tab: what it wants is the tab, and the page that
    // is about a tab is its own.
    focusBrowserTab(tabId: number) {
      this.activeView = 'browser'
      this.unreadCount = 0
      this.browserTabKey = String(tabId)
      this.browserId = null
    },

    setCaptureState(partial: Partial<CaptureState>) {
      this.capture = { ...this.capture, ...partial }
    },

    // The rules the extension filters by, said to whoever is listening. Sent on every state frame:
    // this protocol has no acknowledgement, so repeating them is the only way an extension that has
    // just reconnected learns what it missed.
    async applyCaptureFilters() {
      const filters = useSettings().settings.value?.captureFilters
      if (!filters) return
      await BridgeService.ApplyCaptureFilters(filters)
    },

    // Holding capture down and letting it go: one place for it, because the status bar and the
    // browser page both offer the switch and two copies of it would be two answers.
    async toggleCapture() {
      if (this.capture.paused) await BridgeService.ResumeCapture()
      else await BridgeService.PauseCapture()
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
      // What the line holds is a record from this moment on — it is in the history, and sending does
      // not change the draft — so it stops being work of its own. A line edited after this is work
      // again, which is what the revision moving on says.
      this.lineRevision = this.draft?.revision ?? 0
      // The answer can be back before this call is, and the claim has already adopted it and turned
      // the spinner off: putting the id back would leave the button waiting for what it just got.
      this.claim(id)
      if (this.loading) this.pendingId = id
    },

    // The attempt becomes this pane's: the answer to it will be adopted, and an answer to anything
    // else — a run's request — will not. An answer that was already here when this ran — one that
    // arrived while the call that started the attempt was still on its way back — is adopted now,
    // because this is the first moment the two can be joined. See lib/earlyAnswers: without it the
    // spinner would wait for what has already come.
    claim(id: string) {
      this.mine = [id, ...this.mine].slice(0, 20)
      const early = takeAnswer(id)
      if (!early) return
      if ('failed' in early) this.failSend()
      else void this.finishSend(early.record)
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
//
// It asks for the four fields a seed is made of rather than for a whole record, because the shape
// the pane draws a record in (RecordView) is not the shape the table stores one in — its two bodies
// are text by then, not references to text.
export function recordSeed(
  record: Pick<Record, 'method' | 'url' | 'requestHeaders' | 'requestCookies'>,
  requestBody: string
): Seed {
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

