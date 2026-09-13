import { defineStore } from 'pinia'
import { bodyMissing, recordView, type RecordBodies, type RecordView } from '../lib/requestRecord'
import { findCollection, holderOf, requestCount, trailOf, type Trail } from '../lib/collectionTree'
import type { ChipName } from '../lib/requestSource'
import { t as tr } from '../i18n'
import {
  BodyKind,
  BodySide,
  DraftID,
  RowKind,
  type Auth,
  type Collection,
  type CollectionNode,
  type CollectionRun,
  type CollectionRunResult,
  type CookieRow,
  type FormRow,
  type Record,
  type Row,
  type Scripts,
} from '../../bindings/json-inspector/internal/domain'
import { TextField, type Preview, type RowPatch, type Seed, type TextResult } from '../../bindings/json-inspector/internal/usecase/draft'
import type { RunProgress } from '../../bindings/json-inspector/internal/usecase/collection'
import type { Level } from '../../bindings/json-inspector/internal/usecase/scripting'
import type { NodeEditor } from '../../bindings/json-inspector/internal/transport/wails/models'
import {
  CollectionsService,
  DraftService,
  RecordsService,
  ScriptingService,
} from '../../bindings/json-inspector/internal/transport/wails'

// How long the window holds a text it is typing before handing it over — the same pause the command
// line uses, and for the same reason: the draft is Go's, so every keystroke would otherwise be a
// write on the other side of the boundary.
export const FLUSH_MS = 400

// The saved requests, mirrored. Go owns the tree and the request a card is editing; what the window
// owns is what it is typing, which rows are open, and what is selected.
export const useCollectionsStore = defineStore('collections', {
  state: () => ({
    tree: [] as Collection[],
    selectedId: null as string | null,
    expanded: {} as { [id: string]: boolean },
    filter: '',

    // The card: the node that is open, together with the draft Go opened on it.
    editor: null as NodeEditor | null,
    // Whether the card holds edits the collection does not. The window's bookkeeping: opening a node
    // clears it and one edit sets it, and it is what the unsaved-changes alert asks about.
    dirty: false,

    // The two texts the card is typing, and the revisions that say whose answer is whose.
    urlText: '',
    bodyText: '',
    urlRev: 0,
    bodyRev: 0,
    bufferedUrl: false,
    bufferedBody: false,
    flushTimer: null as ReturnType<typeof setTimeout> | null,

    // The response of the card's own last send.
    record: null as Record | null,
    bodies: {} as RecordBodies,
    pendingId: null as string | null,
    loading: false,
    // The attempts this card started: a record is adopted only if it answers one of them, so a run
    // going on beside it does not land in this pane.
    mine: [] as string[],

    // The run: what the last one came to, and how far the one that is going has got.
    lastRun: null as CollectionRun | null,
    running: null as { done: number; total: number; name: string } | null,

    // The code the selected level runs of its own — null when it has none, which is a level that
    // inherits — and everything that runs around it, outermost first. The id says which level the
    // pair is about: the selection can move while the answer travels, and one level's code shown as
    // another level's is worse than a box that is still empty.
    scriptsFor: null as string | null,
    scripts: null as Scripts | null,
    chain: [] as Level[],

    // A collection made by a button that has no row to type in yet: the tree names it as soon as the
    // row exists, because a new collection is a name the user is about to give rather than one.
    pendingRename: null as string | null,

    openChip: null as ChipName | null,
    inspector: { open: false, path: null as string | null, width: 300 },

    // The alert standing between a click and a card with unsaved edits. Whatever asked is waiting on
    // the answer, and the window draws it from here because the asker is not always the same view.
    pendingLeave: null as null | { name: string; resolve: (ok: boolean) => void },
  }),

  getters: {
    trail(state): Trail | null {
      return trailOf(state.tree, state.selectedId)
    },
    selected(): CollectionNode | null {
      return this.trail?.node ?? null
    },
    // A request opens as a card; a collection opens as its own overview.
    cardOpen(): boolean {
      return this.selected !== null
    },
    collectionId(): string | null {
      return holderOf(this.trail)?.id ?? null
    },
    // Whether the open request takes its authorization from the levels above it. A card does: it is
    // one node of a tree, and the design gives it «Наследовать» where the command line has «Нет».
    canInherit(): boolean {
      return this.cardOpen
    },
    // What those levels answer with — the nearest one that set something, or nothing at all. The chip
    // says what inheriting would mean here instead of leaving the token a blank.
    inheritedAuth(): Auth | null {
      const trail = this.trail
      if (!trail) return null
      for (const level of [...trail.ancestors].reverse()) {
        if (level.auth) return level.auth
      }
      return null
    },
    // What the open level is called: the status bar names it when the level is what is on screen,
    // and the path to it when a card inside is.
    levelName(): string {
      const trail = this.trail
      if (!trail) return ''
      return trail.node?.name ?? trail.collection?.name ?? ''
    },
    breadcrumbs(): { id: string; name: string }[] {
      const trail = this.trail
      if (!trail) return []
      const crumbs = trail.ancestors.map((level) => ({ id: level.id, name: level.name }))
      if (trail.collection) crumbs.push({ id: trail.collection.id, name: trail.collection.name })
      if (trail.node) crumbs.push({ id: trail.node.id, name: trail.node.name })
      return crumbs
    },
    // What running the selection would send, which is the number the overview counts.
    selectedRequestCount(): number {
      const trail = this.trail
      if (!trail) return 0
      if (trail.node) return 1
      return trail.collection ? requestCount(trail.collection) : 0
    },
    // The level whose code the editor shows: a card edits the node it opened, and the overview the
    // collection or the folder that is selected.
    scriptsLevel(state): string | null {
      return state.selectedId
    },
    // What a run of the current selection is started from. A collection runs everything inside it,
    // the collections inside it included, so it answers with the empty id: there is nothing narrower
    // to name. A request is the one request it is.
    runNodeId(): string {
      return this.selected?.id ?? ''
    },

    // ---- the card's request, read the way the command line reads it --------
    method(state): string {
      return state.editor?.state.draft.method ?? 'GET'
    },
    url(state): string {
      return state.urlText
    },
    body(state): string {
      return state.bodyText
    },
    params(state): Row[] {
      return state.editor?.state.draft.params ?? []
    },
    headers(state): Row[] {
      return state.editor?.state.draft.headers ?? []
    },
    cookies(state): CookieRow[] {
      return state.editor?.state.draft.cookies ?? []
    },
    auth(state): Auth {
      return state.editor?.state.draft.auth ?? { type: 'none' as Auth['type'], token: '' }
    },
    preview(state): Preview {
      return state.editor?.state.preview ?? ({ missing: [] } as Preview)
    },
    missingVars(): string[] {
      return this.preview.missing ?? []
    },
    enabledParamsCount(state): number {
      return (state.editor?.state.draft.params ?? []).filter((p) => p.enabled && p.name.trim()).length
    },
    enabledHeadersCount(state): number {
      return (state.editor?.state.draft.headers ?? []).filter((h) => h.enabled && h.name.trim()).length
    },
    bodyKind(state): BodyKind {
      return state.editor?.state.draft.bodyKind ?? BodyKind.BodyRaw
    },
    form(state): FormRow[] {
      return state.editor?.state.draft.form ?? []
    },
    bodyFile(state): string {
      return state.editor?.state.draft.bodyFile ?? ''
    },
    // The same rule the command line answers with: a card's chip must go solid on exactly what makes
    // the command line's go solid.
    hasBody(state): boolean {
      const draft = state.editor?.state.draft
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
    response(state): RecordView | null {
      return state.record ? recordView(state.record, state.bodies) : null
    },
  },

  actions: {
    // ---- the tree ---------------------------------------------------------

    async load() {
      this.applyTree((await CollectionsService.Tree()) ?? [])
    },

    // Every change to the tree answers with the whole tree, so the mirror is replaced rather than
    // patched: the order rows are drawn in is Go's, and a splice here would be a second opinion.
    applyTree(tree: Collection[]) {
      this.tree = tree
      // A selection naming something the tree no longer has — a folder just deleted — is a card with
      // nothing behind it.
      if (this.selectedId && !this.trail) this.clearSelection()
    },

    async createCollection(name: string) {
      const tree = (await CollectionsService.CreateCollection(name, '')) ?? []
      this.applyTree(tree)
      // A new collection is what the user is looking at, so it becomes the selection. It is the last
      // one with that name: Go appends, and the id is Go's to mint.
      const created = [...tree].reverse().find((c) => c.name === name)
      if (!created) return
      await this.select(created.id)
      this.pendingRename = created.id
    },

    async createNode(collectionId: string, name: string, method = 'GET') {
      const created = await CollectionsService.CreateNode({ collectionId, name, method })
      this.applyTree(created.tree ?? [])
      this.expanded[collectionId] = true
      // A new request opens straight away: it was made to be filled in.
      await this.select(created.node.id)
    },

    // What a drop in the tree calls. A row the drop ended up in the same collection it came from is
    // still a move: a request dropped above its neighbour is the order changing and nothing else.
    async moveNode(id: string, collectionId: string, position: number) {
      this.applyTree((await CollectionsService.MoveNode(id, collectionId, position)) ?? [])
    },

    // A collection dropped into another one, or back out at the top level when the parent is empty.
    async moveCollection(id: string, parentId: string, position: number) {
      this.applyTree((await CollectionsService.MoveCollection(id, parentId, position)) ?? [])
    },

    // Saving from the command line copies what is composed into a collection. The draft is not
    // touched: saving a copy is not a move, and what is being composed stays where it is.
    async saveDraft(collectionId: string, name: string) {
      const created = await CollectionsService.SaveDraft(collectionId, name)
      this.applyTree(created.tree ?? [])
      this.expanded[collectionId] = true
    },

    // A collection travels as a file: the window asks Go for an import, and Go reads the file. A
    // cancelled dialog answers with nothing — neither a change nor a failure.
    async importFile(): Promise<string | null> {
      const tree = await CollectionsService.ImportFile(tr('files.importCollection'))
      if (!tree) return null
      this.applyTree(tree)
      // What was imported is the last collection: Go appends, and the file's name is its id-less
      // introduction to the tree.
      const imported = tree[tree.length - 1]
      if (imported) await this.select(imported.id)
      return imported?.name ?? ''
    },

    // Export answers whether anything was written; a cancelled save dialog is not an error and the
    // window says nothing about it.
    async exportFile(id: string): Promise<boolean> {
      return CollectionsService.ExportFile(tr('files.exportCollection'), id)
    },

    async rename(id: string, name: string) {
      this.applyTree((await CollectionsService.Rename(id, name)) ?? [])
    },

    // A description written in the header is saved the way a rename is: the answer is the whole tree,
    // and the tree is where the header reads the line back from.
    async describe(id: string, description: string) {
      this.applyTree((await CollectionsService.Describe(id, description)) ?? [])
    },

    // What the «Авторизация» tab writes: the level's own auth, which everything inside inherits.
    // «Нет» is the same call with an empty auth — Go stores that as no answer at all.
    async saveAuth(id: string, auth: Auth) {
      this.applyTree((await CollectionsService.SaveAuth(id, auth)) ?? [])
    },

    async duplicate(id: string) {
      this.applyTree((await CollectionsService.Duplicate(id, tr('collections.copySuffix'))) ?? [])
    },

    async remove(id: string) {
      this.applyTree((await CollectionsService.Delete(id)) ?? [])
    },

    // A row that closes takes what is inside it with it: a level left open inside a closed row is not a
    // state anybody chose to keep, and the row reopened a minute later should look the way it did the
    // first time rather than unfold whatever happened to be open inside it back then.
    toggleExpand(id: string) {
      const open = !this.expanded[id]
      this.expanded[id] = open
      if (open) return

      const collection = findCollection(this.tree, id)
      if (collection) this.closeBelow(collection)
    },

    // Every collection under one row, down to the leaves. A request holds nothing, so the walk goes
    // through the collections and stops there.
    closeBelow(collection: Collection) {
      this.expanded[collection.id] = false
      for (const child of collection.children ?? []) this.closeBelow(child)
    },

    setFilter(text: string) {
      this.filter = text
    },

    clearSelection() {
      this.selectedId = null
      this.editor = null
      this.dirty = false
      this.record = null
      this.bodies = {}
      this.lastRun = null
    },

    // ---- the card ---------------------------------------------------------

    // What a click in the tree does: a request opens a card, a collection opens its overview.
    async select(id: string) {
      this.selectedId = id
      if (this.selected) {
        await this.openNode(id)
        return
      }

      this.clearFlush()
      this.editor = null
      this.dirty = false
      this.record = null
      this.bodies = {}
      this.lastRun = await CollectionsService.LastRun(this.collectionId ?? '', this.runNodeId)
    },

    async openNode(id: string) {
      this.clearFlush()
      const editor = await CollectionsService.OpenNode(id)
      this.editor = editor
      this.dirty = false
      this.urlText = editor.state.draft.url
      this.bodyText = editor.state.draft.body
      this.urlRev = 0
      this.bodyRev = 0
      this.bufferedUrl = false
      this.bufferedBody = false
      this.record = null
      this.bodies = {}
      this.lastRun = null
    },

    // Saving is its own gesture: sending a card's request does not write it into the collection.
    async saveNode() {
      const id = this.editor?.node.id
      if (!id) return
      await this.flush()
      this.applyEditor(await CollectionsService.SaveNode(id))
    },

    applyEditor(editor: NodeEditor) {
      this.editor = editor
      this.dirty = false
      this.urlText = editor.state.draft.url
      this.bodyText = editor.state.draft.body
      this.bufferedUrl = false
      this.bufferedBody = false
      this.applyTree(editor.tree ?? [])
    },

    draftId(): DraftID | null {
      return this.editor?.state.draft.id ?? null
    },

    apply(state: NodeEditor['state']) {
      if (!this.editor) return
      this.editor.state = state
      if (!this.bufferedUrl) this.urlText = state.draft.url
      if (!this.bufferedBody) this.bodyText = state.draft.body
      this.dirty = true
    },

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
      this.dirty = true
      this.scheduleFlush()
    },

    setBody(text: string) {
      this.bodyText = text
      this.bodyRev += 1
      this.bufferedBody = true
      this.dirty = true
      this.scheduleFlush()
    },

    scheduleFlush() {
      if (this.flushTimer) clearTimeout(this.flushTimer)
      this.flushTimer = setTimeout(() => void this.flush(), FLUSH_MS)
    },

    clearFlush() {
      if (this.flushTimer) clearTimeout(this.flushTimer)
      this.flushTimer = null
    },

    async flush() {
      this.clearFlush()
      const id = this.draftId()
      if (!id) return
      if (this.bufferedUrl) {
        const rev = this.urlRev
        this.applyText(await DraftService.SetText(id, { field: TextField.FieldURL, text: this.urlText, rev }))
      }
      if (this.bufferedBody) {
        const rev = this.bodyRev
        this.applyText(await DraftService.SetText(id, { field: TextField.FieldBody, text: this.bodyText, rev }))
      }
    },

    async setMethod(method: string) {
      const id = this.draftId()
      if (id) this.apply(await DraftService.SetMethod(id, method))
    },

    async setAuth(auth: Auth) {
      const id = this.draftId()
      if (id) this.apply(await DraftService.SetAuth(id, auth))
    },

    async setBodyKind(kind: BodyKind) {
      const id = this.draftId()
      if (id) this.apply(await DraftService.SetBodyKind(id, kind))
    },

    async setBodyFile(path: string) {
      const id = this.draftId()
      if (id) this.apply(await DraftService.SetBodyFile(id, path))
    },

    async pickBodyFile(): Promise<string> {
      return await DraftService.PickBodyFile(tr('files.bodyFile'), tr('files.allFiles'))
    },

    async addRow(kind: RowKind): Promise<string> {
      const id = this.draftId()
      if (!id) return ''
      const before = this.idsOf(kind)
      this.apply(await DraftService.AddRow(id, kind))
      return this.idsOf(kind).find((row) => !before.includes(row)) ?? ''
    },

    async removeRow(kind: RowKind, rowId: string) {
      const id = this.draftId()
      if (id) this.apply(await DraftService.RemoveRow(id, kind, rowId))
    },

    async patchRow(kind: RowKind, rowId: string, patch: RowPatch) {
      const id = this.draftId()
      if (id) this.apply(await DraftService.PatchRow(id, kind, rowId, patch))
    },

    async toggleRow(kind: RowKind, rowId: string, enabled: boolean) {
      await this.patchRow(kind, rowId, { enabled })
    },

    idsOf(kind: RowKind): string[] {
      if (kind === RowKind.RowParams) return this.params.map((r) => r.id)
      if (kind === RowKind.RowHeaders) return this.headers.map((r) => r.id)
      return this.cookies.map((r) => r.id ?? '')
    },

    // A whole request handed to the card: a pasted command, or a link followed out of a response.
    async replace(seed: Seed) {
      // A flush of the text being replaced must not land on the draft that replaced it.
      this.clearFlush()
      const id = this.draftId()
      if (!id) return
      this.urlRev += 1
      this.bodyRev += 1
      this.bufferedUrl = false
      this.bufferedBody = false
      this.apply(await DraftService.Replace(id, seed))
    },

    setOpenChip(chip: ChipName | null) {
      this.openChip = chip
    },

    setInspector(partial: Partial<{ open: boolean; path: string | null; width: number }>) {
      this.inspector = { ...this.inspector, ...partial }
    },

    // ---- sending ----------------------------------------------------------

    // A card's request goes out through Go, which is the side that can fill the variables in. The
    // flush first is what makes it the request the user is looking at — and sending does not save.
    async send() {
      await this.flush()
      const id = this.draftId()
      if (!id) return
      this.loading = true
      const sent = await RecordsService.Send(id)
      this.mine = [sent, ...this.mine].slice(0, 20)
      // The record can outrun the call that asked for it: putting the id back would leave the button
      // waiting for what it just got.
      if (this.loading) this.pendingId = sent
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
      await this.showRecord(record)
    },

    // What the pane draws: a record and its bodies. The same call for the answer to a send and for the
    // answer a run left behind — the only difference is who asked.
    async showRecord(record: Record) {
      this.record = record
      this.bodies = {
        request: record.requestBody?.inline,
        response: record.responseBody?.inline,
      }
      if (bodyMissing(record.requestBody)) {
        this.bodies.request = await RecordsService.Body(record.id, BodySide.SideRequest)
      }
      if (bodyMissing(record.responseBody)) {
        this.bodies.response = await RecordsService.Body(record.id, BodySide.SideResponse)
      }
    },

    failSend() {
      this.pendingId = null
      this.loading = false
    },

    // ---- leaving a card with unsaved edits --------------------------------

    // Anything that would take the user away from the open card asks this first: it answers at once
    // when there is nothing to lose, and otherwise waits for the alert to be answered.
    askUnsaved(): Promise<boolean> {
      if (!this.dirty) return Promise.resolve(true)
      const name = this.editor?.node.name ?? this.selected?.name ?? tr('collections.unnamedRequest')
      return new Promise((resolve) => {
        this.pendingLeave = { name, resolve }
      })
    },

    async answerUnsaved(action: 'save' | 'discard' | 'cancel') {
      const pending = this.pendingLeave
      if (!pending) return
      this.pendingLeave = null

      if (action === 'cancel') {
        pending.resolve(false)
        return
      }
      if (action === 'save') {
        await this.saveNode()
      } else {
        // «Не сохранять» бросает правку: черновик в Go остаётся, но узел его больше не увидит —
        // открытие узла заново начинается с того, что записано в дереве.
        this.dirty = false
      }
      pending.resolve(true)
    },

    // ---- the run ----------------------------------------------------------

    async run(nodeId: string, name: string) {
      const collectionId = this.collectionId
      if (!collectionId) return
      // The counters start at nothing: the first progress event is what says how many requests there
      // are, and guessing one here would show a total the run may not have.
      this.running = { done: 0, total: 0, name }
      try {
        await CollectionsService.Run(collectionId, nodeId)
      } catch (error) {
        this.running = null
        throw error
      }
    },

    async stop() {
      await CollectionsService.Stop()
    },

    // One request of a run has come back. The counters feed the status bar, and the row itself is what
    // fills the pane as the run goes: a collection of fifty requests is a minute of watching, and a
    // list that appears only at the end is a list that says nothing while it matters.
    applyRunProgress(progress: RunProgress) {
      if (!this.running) return
      this.running = { ...this.running, done: progress.done, total: progress.total }

      // A row is drawn by the level it was run from: a window that has since opened another collection
      // is not this run's, and neither was it before.
      if (progress.collectionId !== this.collectionId || progress.nodeId !== this.runNodeId) return

      const run = this.lastRun
      if (!run || run.id !== progress.runId) {
        // The run that is going has no rows of its own yet — and the ones on screen belong to the
        // previous run, which is a different thing to be looking at.
        this.lastRun = {
          id: progress.runId,
          collectionId: progress.collectionId,
          nodeId: progress.nodeId,
          startedAt: Date.now(),
          finishedAt: 0,
          durationUs: 0,
          passed: 0,
          failed: 0,
          results: [progress.result],
        }
        return
      }
      this.lastRun = { ...run, results: [...(run.results ?? []), progress.result] }
    },

    // A row of the last run opens the request it names, with the answer that run got: the same gesture
    // a history row makes, and the reason the result carries the record it produced. A request that
    // never went out — one a script kept back, one whose node was deleted — opens as a plain card.
    async openRunResult(result: CollectionRunResult) {
      await this.select(result.nodeId)
      if (!result.recordId) return
      try {
        await this.showRecord(await RecordsService.Record(result.recordId))
      } catch {
        // The record is gone — pruned history, or a run from a database that has been cleared — and the
        // request itself is still worth opening. Losing the answer is not losing the row.
      }
    },

    // A run of another node is not this overview's: the row is drawn from what was run here.
    applyRunFinished(run: CollectionRun) {
      this.running = null
      if (run.collectionId !== this.collectionId || (run.nodeId ?? '') !== this.runNodeId) return
      this.lastRun = run
    },

    // ---- the code of a level ----------------------------------------------

    // What the level runs itself and what runs around it. Both are read together because they are one
    // answer: an empty editor is not empty code, it is the code above it.
    async loadScripts() {
      const id = this.selectedId
      if (!id) return
      const scripts = await ScriptingService.Scripts(id)
      const chain = (await ScriptingService.Chain(id)) ?? []
      this.scriptsFor = id
      this.scripts = scripts
      this.chain = chain
    },

    // A level adds code and never cancels it: what it has of its own runs after everything above it,
    // so an editor that has been cleared means "нечего добавить" — the level goes back to inheriting.
    async saveScripts(pre: string, post: string) {
      const id = this.selectedId
      if (!id) return
      const written = pre.trim() || post.trim() ? { pre, post } : null
      const saved = await ScriptingService.SaveScripts(id, written)
      // A level that just gained or lost its code is a level that just entered or left the chain.
      const chain = (await ScriptingService.Chain(id)) ?? []
      this.scriptsFor = id
      this.scripts = saved
      this.chain = chain
    },

    // What the level would run for one half if its own editor stayed empty: the nearest code above
    // it. It is what the editor shows greyed out, so an empty box is not a box that does nothing.
    inheritedScript(scope: 'pre' | 'post'): { text: string; name: string } | null {
      for (let i = this.chain.length - 1; i >= 0; i -= 1) {
        const level = this.chain[i]
        if (level.nodeId === this.selectedId) continue
        const text = scope === 'pre' ? level.scripts?.pre : level.scripts?.post
        if (text && text.trim()) return { text, name: level.name }
      }
      return null
    },
  },
})

