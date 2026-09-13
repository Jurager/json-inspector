import { defineStore } from 'pinia'
import { bodyMissing, recordView, type RecordBodies, type RecordView } from '../lib/requestRecord'
import { requestCount, trailOf, type Trail } from '../lib/collectionTree'
import {
  BodySide,
  DraftID,
  RowKind,
  type Auth,
  type Collection,
  type CollectionNode,
  type CollectionRun,
  type CookieRow,
  type NodeKind,
  type Record,
  type Row,
} from '../../bindings/json-inspector/internal/domain'
import { TextField, type Preview, type RowPatch, type Seed, type TextResult } from '../../bindings/json-inspector/internal/usecase/draft'
import type { NodeEditor } from '../../bindings/json-inspector/internal/transport/wails/models'
import { CollectionsService, DraftService, RecordsService } from '../../bindings/json-inspector/internal/transport/wails'

// How long the window holds a text it is typing before handing it over — the same pause the command
// line uses, and for the same reason: the draft is Go's, so every keystroke would otherwise be a
// write on the other side of the boundary.
const FLUSH_MS = 400

export type ChipName = 'params' | 'headers' | 'auth' | 'body'

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
    // A request opens as a card; a collection or a folder opens as its own overview.
    cardOpen(): boolean {
      return this.selected?.kind === 'request'
    },
    collectionId(): string | null {
      return this.trail?.collection.id ?? null
    },
    breadcrumbs(): { id: string; name: string }[] {
      const trail = this.trail
      if (!trail) return []
      const crumbs = [{ id: trail.collection.id, name: trail.collection.name }]
      for (const node of trail.ancestors) crumbs.push({ id: node.id, name: node.name })
      if (trail.node) crumbs.push({ id: trail.node.id, name: trail.node.name })
      return crumbs
    },
    // What running the selection would send, which is the number the overview counts.
    selectedRequestCount(): number {
      const trail = this.trail
      if (!trail) return 0
      if (trail.node) return trail.node.kind === 'request' ? 1 : requestCount(trail.node)
      return (trail.collection.items ?? []).reduce((total: number, node: CollectionNode) => {
        return total + (node.kind === 'request' ? 1 : requestCount(node))
      }, 0)
    },
    // The node a run of the current selection is started from: a folder runs its subtree, and the
    // collection itself is the empty id — a saved folder is a row of its own, not the absence of one.
    runNodeId(): string {
      return this.selected?.kind === 'folder' ? (this.selected.id ?? '') : ''
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
    bodyDisabled(): boolean {
      return this.method === 'GET' || this.method === 'HEAD'
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

    async createNode(collectionId: string, parentId: string, kind: NodeKind, name: string, method = 'GET') {
      const created = await CollectionsService.CreateNode({ collectionId, parentId, kind, name, method })
      this.applyTree(created.tree ?? [])
      this.expanded[parentId || collectionId] = true
      // A new request opens straight away: it was made to be filled in.
      await this.select(created.node.id)
    },

    // Saving from the command line copies what is composed into a collection. The draft is not
    // touched: saving a copy is not a move, and what is being composed stays where it is.
    async saveDraft(collectionId: string, parentId: string, name: string) {
      const created = await CollectionsService.SaveDraft(collectionId, parentId, name)
      this.applyTree(created.tree ?? [])
      this.expanded[parentId || collectionId] = true
    },

    // A collection travels as a file: the window asks Go for an import, and Go reads the file. A
    // cancelled dialog answers with nothing — neither a change nor a failure.
    async importFile(): Promise<string | null> {
      const tree = await CollectionsService.ImportFile()
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
      return CollectionsService.ExportFile(id)
    },

    async rename(id: string, name: string) {
      this.applyTree((await CollectionsService.Rename(id, name)) ?? [])
    },

    async duplicate(id: string) {
      this.applyTree((await CollectionsService.Duplicate(id)) ?? [])
    },

    async remove(id: string) {
      this.applyTree((await CollectionsService.Delete(id)) ?? [])
    },

    toggleExpand(id: string) {
      this.expanded[id] = !this.expanded[id]
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

    // What a click in the tree does: a request opens a card, everything else opens its overview.
    async select(id: string) {
      this.selectedId = id
      if (this.selected?.kind === 'request') {
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
      const name = this.editor?.node.name ?? this.selected?.name ?? 'запрос'
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

    applyRunProgress(done: number, total: number) {
      if (!this.running) return
      this.running = { ...this.running, done, total }
    },

    // A run of another node is not this overview's: the row is drawn from what was run here.
    applyRunFinished(run: CollectionRun) {
      this.running = null
      if (run.collectionId !== this.collectionId || (run.nodeId ?? '') !== this.runNodeId) return
      this.lastRun = run
    },
  },
})

