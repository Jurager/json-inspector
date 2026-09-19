import { defineStore } from 'pinia'
import { bodyMissing, recordView, type RecordBodies, type RecordView } from '../lib/requestRecord'
import { findCollection, holderOf, requestCount, trailOf, type Trail } from '../lib/collectionTree'
import { takeAnswer } from '../lib/earlyAnswers'
import { NO_AUTH } from '../lib/requestSource'
import type { ChipName } from '../lib/requestSource'
import { abandoned, owed, settled, typed } from '../lib/draftBuffer'
import { asked } from './calls'
import { t as tr } from '../i18n'
import {
  BodyKind,
  BodySide,
  DraftID,
  RowKind,
  type Auth,
  type AuthToken,
  type Collection,
  type CollectionNode,
  type CollectionRun,
  type CollectionRunResult,
  type CookieRow,
  type FormRow,
  ProjectedRow,
  type Record,
  type Row,
  type Scripts,
  type Variable,
} from '../../bindings/json-inspector/internal/domain'
import {
  CommandKind,
  TextField,
  type CommandResult,
  type Preview,
  type RowPatch,
  type Seed,
  type State,
  type TextResult,
} from '../../bindings/json-inspector/internal/usecase/draft'
import type { RunProgress } from '../../bindings/json-inspector/internal/usecase/collection'
import type { Level } from '../../bindings/json-inspector/internal/usecase/scripting'
import type { NodeEditor } from '../../bindings/json-inspector/internal/transport/wails/models'
import {
  CollectionsService,
  DraftService,
  RecordsService,
  ScriptingService,
} from '../../bindings/json-inspector/internal/transport/wails'

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
    // The same idea from the other side: the key that makes a request knows which level it goes in,
    // and the tree opens its row for naming — the gesture the context menu's own item makes.
    pendingCreate: null as string | null,

    openChip: null as ChipName | null,
    inspector: { open: false, path: null as string | null, width: 330 },

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
    // The level a new row would join: a folder's own row names the folder, and a request's row names
    // the collection it sits in, because a request holds nothing. The context menu's first item and
    // the key that makes a request both ask this, so both make it in the same place.
    selectedLevel(): string {
      return (this.selected ? this.collectionId : this.selectedId) ?? ''
    },

    // Whether the open request takes its authorization from the levels above it. A card does: it is
    // one node of a tree, and the design gives it «Наследовать» where the command line has «Нет».
    canInherit(): boolean {
      return this.cardOpen
    },
    // What those levels answer with — the nearest one that gave a credential, or nothing at all.
    // The chip says what inheriting would mean here instead of leaving the token a blank, and the
    // answer comes from Go with every other: the walk past a level that said «нет» is the tree's
    // rule, and repeating it here would be a second place for it to mean something else.
    inheritedAuth(state): Auth | null {
      return state.editor?.state.inherited ?? null
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
      return state.editor?.state.draft.auth ?? NO_AUTH
    },
    // Empty means this card follows whatever the window is on; a pin is the card's own answer.
    environmentId(state): string {
      return state.editor?.state.draft.environmentId ?? ''
    },
    // The rows the authorization puts in the lists, worked out by Go. Empty until the schemas load,
    // which is a moment nobody sees: nothing to project means no authorization is set yet.
    projected(state): ProjectedRow[] {
      return state.editor?.state.projected ?? []
    },
    // The state of a token somebody else issued, for the schemes that have one. Absent means the
    // scheme carries what the user typed, and there is nothing to say about it.
    token(state): AuthToken | null {
      return state.editor?.state.token ?? null
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
      const tree = await asked(CollectionsService.Tree(), 'collections.readFailed')
      if (!tree) return
      this.applyTree(tree ?? [])
    },

    // What a workspace switch leaves behind. The tree arrives next and replaces what is drawn; what
    // has to go now is everything that names something of the workspace being left — a selected node,
    // a card open on it, a run of a collection that is not in the tree any more.
    forget() {
      this.tree = []
      this.selectedId = null
      this.expanded = {}
      this.editor = null
      this.dirty = false
      this.record = null
      this.bodies = {}
      this.lastRun = null
      this.running = null
      this.pendingId = null
      this.loading = false
      this.filter = ''
    },

    // Every change to the tree answers with the whole tree, so the mirror is replaced rather than
    // patched: the order rows are drawn in is Go's, and a splice here would be a second opinion.
    applyTree(tree: Collection[]) {
      this.tree = tree
      // A selection naming something the tree no longer has — a folder just deleted — is a card with
      // nothing behind it.
      if (this.selectedId && !this.trail) this.clearSelection()
    },

    // A folder is a collection with a parent, so making one is this call with the level it goes in.
    // Empty is the top of the tree, which is where the panel's «+» puts one.
    async createCollection(name: string, parentId = '') {
      const tree = await asked(
        CollectionsService.CreateCollection(name, '', parentId),
        'collections.saveFailed'
      )
      if (!tree) return
      this.applyTree(tree ?? [])
      // A new collection is what the user is looking at, so it becomes the selection. It is the last
      // one with that name: Go appends, and the id is Go's to mint.
      const created = [...tree].reverse().find((c) => c.name === name)
      if (!created) return
      await this.select(created.id)
      this.pendingRename = created.id
    },

    async createNode(collectionId: string, name: string, method = 'GET') {
      const created = await asked(
        CollectionsService.CreateNode({ collectionId, name, method }),
        'collections.saveFailed'
      )
      if (!created) return
      this.applyTree(created.tree ?? [])
      this.expanded[collectionId] = true
      // A new request opens straight away: it was made to be filled in.
      await this.select(created.node.id)
    },

    // What a drop in the tree calls. A row the drop ended up in the same collection it came from is
    // still a move: a request dropped above its neighbour is the order changing and nothing else.
    async moveNode(id: string, collectionId: string, position: number) {
      const tree = await asked(CollectionsService.MoveNode(id, collectionId, position), 'collections.saveFailed')
      if (tree) this.applyTree(tree)
    },

    // A collection dropped into another one, or back out at the top level when the parent is empty.
    async moveCollection(id: string, parentId: string, position: number) {
      const tree = await asked(CollectionsService.MoveCollection(id, parentId, position), 'collections.saveFailed')
      if (tree) this.applyTree(tree)
    },

    // Saving from the command line copies what is composed into a collection. The draft is not
    // touched: saving a copy is not a move, and what is being composed stays where it is.
    async saveDraft(collectionId: string, name: string) {
      const created = await asked(
        CollectionsService.SaveDraft(collectionId, name),
        'collections.saveFailed'
      )
      if (!created) return
      this.applyTree(created.tree ?? [])
      this.expanded[collectionId] = true
    },

    // A collection travels as a file: the window asks Go for an import, and Go reads the file. A
    // cancelled dialog answers with nothing — neither a change nor a failure.
    async importFile(): Promise<string | null> {
      const tree = await asked(
        CollectionsService.ImportFile(tr('files.importCollection')),
        'collections.importFailed'
      )
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
      const tree = await asked(CollectionsService.Rename(id, name), 'collections.saveFailed')
      if (tree) this.applyTree(tree)
    },

    // A description written in the header is saved the way a rename is: the answer is the whole tree,
    // and the tree is where the header reads the line back from.
    async describe(id: string, description: string) {
      const tree = await asked(CollectionsService.Describe(id, description), 'collections.saveFailed')
      if (tree) this.applyTree(tree)
    },

    // The `{{tokens}}` the level answers for everything inside it. The set travels whole, as the editor
    // holds it: Go numbers it, mints the ids of rows that arrived without one, and refuses a secret —
    // a collection is what gets exported and handed on.
    async saveVariables(id: string, variables: Variable[]) {
      const tree = await asked(CollectionsService.SaveVariables(id, variables), 'collections.saveFailed')
      if (tree) this.applyTree(tree)
    },

    // What the «Авторизация» tab writes: the level's own auth, which everything inside inherits.
    // «Нет» is the same call with an empty auth — Go stores that as no answer at all.
    async saveAuth(id: string, auth: Auth) {
      const tree = await asked(CollectionsService.SaveAuth(id, auth), 'collections.saveFailed')
      if (tree) this.applyTree(tree)
    },

    async duplicate(id: string) {
      const tree = await asked(CollectionsService.Duplicate(id, tr('collections.copySuffix')), 'collections.saveFailed')
      if (tree) this.applyTree(tree)
    },

    async remove(id: string) {
      const tree = await asked(CollectionsService.Delete(id), 'collections.saveFailed')
      if (tree) this.applyTree(tree)
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

      abandoned(this)
      this.editor = null
      this.dirty = false
      this.record = null
      this.bodies = {}
      const run = await asked(
        CollectionsService.LastRun(this.collectionId ?? '', this.runNodeId),
        'collections.readFailed'
      )
      if (run === undefined) return
      this.lastRun = run
    },

    async openNode(id: string) {
      abandoned(this)
      const editor = await asked(
        CollectionsService.OpenNode(id),
        'collections.readFailed'
      )
      if (!editor) return
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
      const editor = await asked(CollectionsService.SaveNode(id), 'collections.saveFailed')
      if (editor) this.applyEditor(editor)
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

    // An edit of the card's request, from the click to what the window draws — the command line's own
    // shape, and guarded for the same reason: Go's answer is the only state this side has, so an edit
    // Go refused leaves the card exactly as it was and says why. Answers whether anything landed.
    async edit(call: Promise<NodeEditor['state']>): Promise<boolean> {
      const state = await asked(call, 'request.editFailed')
      if (!state) return false
      this.apply(state)
      return true
    },

    // The card's own answer, asked again for the same reason the command line asks: the variables it
    // names live in a window of their own, and what this holds goes stale while that window is open.
    // A new editor object rather than a field written into the old one, so nothing is mutated in place.
    async refreshPreview() {
      const id = this.draftId()
      if (!id || !this.editor) return
      // A read behind the button rather than behind the person: a preview that could not be asked
      // for leaves the send-block as it was, and the send itself refuses a missing name properly.
      const state = await asked(DraftService.Snapshot(id), 'request.readFailed')
      if (!state) return
      this.editor = { ...this.editor, state: { ...this.editor.state, preview: state.preview } }
    },

    applyText(result: TextResult) {
      if (settled(this, result)) this.apply(result)
    },

    setUrl(text: string) {
      typed(this, TextField.FieldURL, text, () => void this.flush())
      this.dirty = true
    },

    setBody(text: string) {
      typed(this, TextField.FieldBody, text, () => void this.flush())
      this.dirty = true
    },

    async flush() {
      const id = this.draftId()
      const parts = owed(this)
      if (!id) return
      for (const part of parts) {
        const result = await asked(DraftService.SetText(id, part), 'request.editFailed')
        if (result) this.applyText(result)
      }
    },

    async setMethod(method: string) {
      const id = this.draftId()
      if (id) await this.edit(DraftService.SetMethod(id, method))
    },

    async setEnvironmentOverride(envId: string) {
      const id = this.draftId()
      if (id) await this.edit(DraftService.SetEnvironmentOverride(id, envId))
    },

    async setAuth(auth: Auth) {
      const id = this.draftId()
      if (id) await this.edit(DraftService.SetAuth(id, auth))
    },

    async patchDerived(target: RowKind, name: string, value: string) {
      const id = this.draftId()
      if (id) await this.edit(DraftService.PatchDerived(id, target, name, value))
    },

    async removeDerived() {
      const id = this.draftId()
      if (id) await this.edit(DraftService.RemoveDerived(id))
    },

    // «Получить токен» and «Очистить»: the card's own authorization, not the tree's. A level that
    // inherits its answers inherits what they come to as well, and the buttons belong to whoever set
    // them — which is the Auth tab of the collection, not the popover of a request inside it.
    async obtainAuth() {
      const id = this.draftId()
      if (id) await this.edit(DraftService.ObtainAuth(id))
    },

    async forgetAuth() {
      const id = this.draftId()
      if (id) await this.edit(DraftService.ForgetAuth(id))
    },


    async setBodyKind(kind: BodyKind) {
      const id = this.draftId()
      if (id) await this.edit(DraftService.SetBodyKind(id, kind))
    },

    async setBodyFile(path: string) {
      const id = this.draftId()
      if (id) await this.edit(DraftService.SetBodyFile(id, path))
    },

    async pickBodyFile(): Promise<string> {
      const path = await asked(
        DraftService.PickBodyFile(tr('files.bodyFile'), tr('files.allFiles')),
        'files.pickFailed'
      )
      return path ?? ''
    },

    async addRow(kind: RowKind): Promise<string> {
      const id = this.draftId()
      if (!id) return ''
      const before = this.idsOf(kind)
      const landed = await this.edit(DraftService.AddRow(id, kind))
      if (!landed) return ''
      return this.idsOf(kind).find((row) => !before.includes(row)) ?? ''
    },

    async removeRow(kind: RowKind, rowId: string) {
      const id = this.draftId()
      if (id) await this.edit(DraftService.RemoveRow(id, kind, rowId))
    },

    async patchRow(kind: RowKind, rowId: string, patch: RowPatch) {
      const id = this.draftId()
      if (id) await this.edit(DraftService.PatchRow(id, kind, rowId, patch))
    },

    async toggleRow(kind: RowKind, rowId: string, enabled: boolean) {
      await this.patchRow(kind, rowId, { enabled })
    },

    idsOf(kind: RowKind): string[] {
      if (kind === RowKind.RowParams) return this.params.map((r) => r.id)
      if (kind === RowKind.RowHeaders) return this.headers.map((r) => r.id)
      return this.cookies.map((r) => r.id ?? '')
    },

    // A whole request handed to the card: a link followed out of a response, or a record opened in
    // it.
    async replace(seed: Seed) {
      const id = this.draftId()
      if (!id) return
      // A flush of the text being replaced must not land on the draft that replaced it.
      abandoned(this)
      this.urlRev += 1
      this.bodyRev += 1
      await this.edit(DraftService.Replace(id, seed))
    },

    // A command pasted into a card's line. Reading it and handing it to the draft is one call on the
    // other side; what stays here is the buffering, which belongs to the window. See the command
    // line's own pasteCommand for why the reading travels back.
    async pasteCommand(text: string): Promise<CommandResult | null> {
      const id = this.draftId()
      if (!id) return { kind: CommandKind.KindNone } as CommandResult
      abandoned(this)
      this.urlRev += 1
      this.bodyRev += 1

      const pasted = await asked(DraftService.PasteCommand(id, text), 'request.editFailed')
      if (!pasted) return null
      if (pasted.state) this.apply(pasted.state)
      return pasted.reading
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
      // Let out rather than swallowed: the wording of a refusal belongs to the screen the button is
      // on, and it is that screen that catches this and words it. What belongs here is the button
      // not being left waiting for a request that was never started.
      let sent: string
      try {
        sent = await RecordsService.Send(id)
      } catch (error) {
        this.failSend()
        throw error
      }
      // The answer can be back before this call is, and this claim has already adopted it and turned
      // the spinner off by then: putting the id back would leave the button waiting for what it just
      // got.
      this.claim(sent)
      if (this.loading) this.pendingId = sent
    },

    // The attempt becomes the card's, and an answer that was already here — one that arrived while
    // the call that started the attempt was still on its way back — is adopted now, because this is
    // the first moment the two can be joined. See lib/earlyAnswers: without it the spinner would
    // wait for what has already come.
    claim(id: string) {
      this.mine = [id, ...this.mine].slice(0, 20)
      const early = takeAnswer(id)
      if (!early) return
      if ('failed' in early) this.failSend()
      else void this.finishSend(early.record)
    },

    // A cancel that failed is not news: the request is already on its way, and the button has stopped
    // waiting for it either way.
    async cancel() {
      const id = this.pendingId
      this.pendingId = null
      this.loading = false
      if (id) await asked(RecordsService.Cancel(id), 'request.sendFailed')
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
      // Only a body the record did not carry is read by name: one that came inline is already here,
      // and asking for it again would be a call per look at the same row.
      //
      // A row can outlive what it names — the history prunes itself, and another window can clear it
      // — and the refusal is said out loud while the body is left as the record had it: a pane with
      // the answer it did bring beats an empty one.
      if (bodyMissing(record.requestBody)) {
        const request = await asked(
          RecordsService.Body(record.id, BodySide.SideRequest),
          'response.openFailed'
        )
        if (request) this.bodies.request = request
      }
      if (bodyMissing(record.responseBody)) {
        const response = await asked(
          RecordsService.Body(record.id, BodySide.SideResponse),
          'response.openFailed'
        )
        if (response) this.bodies.response = response
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
      await asked(CollectionsService.Stop(), 'collections.saveFailed')
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
          // The environment the run is going out under travels with every row, so a page that draws
          // the run while it is still going says the same thing it will say afterwards.
          environment: progress.environment ?? '',
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
      const scripts = await asked(ScriptingService.Scripts(id), 'request.readFailed')
      const chain = await asked(ScriptingService.Chain(id), 'request.readFailed')
      if (!scripts || !chain) return
      this.scriptsFor = id
      this.scripts = scripts
      this.chain = chain ?? []
    },

    // A level adds code and never cancels it: what it has of its own runs after everything above it,
    // so an editor that has been cleared means "нечего добавить" — the level goes back to inheriting.
    async saveScripts(pre: string, post: string, off: { pre: boolean; post: boolean }) {
      const id = this.selectedId
      if (!id) return
      // A switch on an empty half is nothing to keep: there is no code for it to be about.
      const flags = {
        preOff: off.pre && !!pre.trim(),
        postOff: off.post && !!post.trim(),
      }
      const written = pre.trim() || post.trim() ? { pre, post, ...flags } : null
      const saved = await asked(ScriptingService.SaveScripts(id, written), 'request.editFailed')
      // A level that just gained or lost its code is a level that just entered or left the chain.
      const chain = await asked(ScriptingService.Chain(id), 'request.readFailed')
      if (!saved || !chain) return
      this.scriptsFor = id
      this.scripts = saved
      this.chain = chain ?? []
    },

    // What the level would run for one half if its own editor stayed empty: the nearest code above
    // it. It is what the editor shows greyed out, so an empty box is not a box that does nothing.
    inheritedScript(scope: 'pre' | 'post'): { text: string; name: string } | null {
      for (let i = this.chain.length - 1; i >= 0; i -= 1) {
        const level = this.chain[i]
        if (level.nodeId === this.selectedId) continue
        // A level that switched this half off does not run it, so it is not the code an empty box
        // would fall back to either: offering it would name a script the request never runs.
        const off = scope === 'pre' ? level.scripts?.preOff : level.scripts?.postOff
        if (off) continue
        const text = scope === 'pre' ? level.scripts?.pre : level.scripts?.post
        if (text && text.trim()) return { text, name: level.name }
      }
      return null
    },
  },
})

