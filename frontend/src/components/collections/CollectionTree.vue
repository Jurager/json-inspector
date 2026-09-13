<script setup lang="ts">
import { computed, nextTick, ref, watch, type ComponentPublicInstance } from 'vue'
import Icon from '../ui/Icon.vue'
import { IconButton } from '../ui/button'
import { Input } from '../ui/input'
import {
  ContextMenu,
  ContextMenuTrigger,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
} from '../ui/context-menu'
import DeleteNodeDialog from './DeleteNodeDialog.vue'
import { useCollectionsStore } from '../../stores/collections'
import { filterTree, requestCount } from '../../lib/collectionTree'
import { useToast } from '../../composables/useToast'
import { NodeKind, type Collection, type CollectionNode } from '../../../bindings/json-inspector/internal/domain'

const store = useCollectionsStore()
const toast = useToast()

// One request, one file: the menu exports what it was opened on, and a folder or a collection is
// exported from its own overview.
async function exportNode(row: Row) {
  const written = await store.exportFile(row.id)
  if (written) toast.show(`«${row.name}» сохранён в файл`)
}

// The indent the design gives the three levels: the collection, a folder, and what is inside one.
// Deeper nesting is possible — a folder in a folder — and keeps the last step rather than running
// off the panel.
const INDENT = [0, 24, 40]

// A row of the tree, flattened: the drawing walks a list, and the nesting is what the indent says.
// Expansion is applied here rather than by the tree, so the filter can open a path to a match
// without moving anything the user opened.
interface Row {
  id: string
  kind: CollectionNode['kind'] | 'collection'
  name: string
  method: string
  count: number
  depth: number
  expandable: boolean
  expanded: boolean
  collectionId: string
  parentId: string
}

const query = computed({
  get: () => store.filter,
  set: (value: string) => store.setFilter(value),
})

const searching = computed(() => store.filter.trim().length > 0)

const visible = computed<Row[]>(() => flatten(filterTree(store.tree, store.filter)))

function flatten(tree: Collection[]): Row[] {
  const rows: Row[] = []
  for (const collection of tree) {
    const expanded = searching.value || store.expanded[collection.id]
    rows.push({
      id: collection.id,
      kind: 'collection',
      name: collection.name,
      method: '',
      count: requestCount(collection),
      depth: 0,
      expandable: (collection.items ?? []).length > 0,
      expanded: !!expanded,
      collectionId: collection.id,
      parentId: '',
    })
    if (expanded) rows.push(...children(collection.items ?? [], collection.id, 1))
  }
  return rows
}

function children(nodes: CollectionNode[], collectionId: string, depth: number): Row[] {
  const rows: Row[] = []
  for (const node of nodes) {
    const expanded = searching.value || store.expanded[node.id]
    rows.push({
      id: node.id,
      kind: node.kind,
      name: node.name,
      method: node.method ?? 'GET',
      count: node.kind === 'folder' ? requestCount(node) : 0,
      depth,
      expandable: (node.items ?? []).length > 0,
      expanded: !!expanded,
      collectionId,
      parentId: node.parentId ?? '',
    })
    if (expanded && node.kind === 'folder') rows.push(...children(node.items ?? [], collectionId, depth + 1))
  }
  return rows
}

// The indent a row is drawn at. Deeper nesting keeps the last step rather than running off the
// panel.
function indentDepth(depth: number): string {
  return `${INDENT[Math.min(depth, INDENT.length - 1)]}px`
}

// Where the row being named will land: one level inside whatever it is being added to. The parent
// is looked up among the rows on screen, because that is where its depth is known.
function creatingIndent(parentId: string): string {
  const parent = visible.value.find((row) => row.id === parentId)
  return indentDepth((parent?.depth ?? 0) + 1)
}

// A click selects and opens: a request as a card, a collection or a folder as its overview. What is
// inside a collapsed row is reached by its chevron, which is the gesture the tree has always had.
async function pick(row: Row) {
  if (!(await leave())) return
  await store.select(row.id)
}

// ---- renaming ------------------------------------------------------------

const renamingId = ref<string | null>(null)
const nameDraft = ref('')
const renameInput = ref<HTMLInputElement | null>(null)
const renameInvalid = ref(false)

function setNameInput(el: Element | ComponentPublicInstance | null) {
  renameInput.value = (el as HTMLInputElement | null) ?? null
}

function startRename(row: Row) {
  renamingId.value = row.id
  nameDraft.value = row.name
  renameInvalid.value = false
  nextTick(() => {
    focusNextFrame(renameInput.value)
    renameInput.value?.select()
  })
}

async function commitRename() {
  const id = renamingId.value
  if (!id) return
  const name = nameDraft.value.trim()
  if (!name) {
    // An empty name is refused by Go, and saying so here keeps the row open on the text instead of
    // closing it and letting a call fail.
    renameInvalid.value = true
    renameInput.value?.focus()
    return
  }
  renamingId.value = null
  renameInvalid.value = false
  await store.rename(id, name)
}

function cancelRename() {
  renamingId.value = null
  renameInvalid.value = false
}

// The field blurs when the row it was in goes away — Escape unmounts it — and committing there
// would rename a row the user just decided not to.
function commitRenameFrom(id: string) {
  if (renamingId.value === id) void commitRename()
}

function onRenameKeydown(e: KeyboardEvent) {
  e.stopPropagation()
  if (e.key === 'Enter') {
    e.preventDefault()
    void commitRename()
  } else if (e.key === 'Escape') {
    e.preventDefault()
    cancelRename()
  }
}

// ---- creating ------------------------------------------------------------

// The row being named right now: the design creates straight in the tree, the way the environments
// sheet does, so the name is typed where the row will be.
const creating = ref<{ parentId: string; collectionId: string; kind: NodeKind; method: string } | null>(null)
const creatingName = ref('')
const creatingInput = ref<HTMLInputElement | null>(null)
const creatingInvalid = ref(false)

function setCreatingInput(el: Element | ComponentPublicInstance | null) {
  creatingInput.value = (el as HTMLInputElement | null) ?? null
}

// A field that appears where a menu item was clicked cannot be focused in the same tick: the menu is
// still closing, and whatever it does with focus on the way out lands after this. The next frame is
// the first moment the caret stays where it was put.
function focusNextFrame(el: HTMLInputElement | null) {
  requestAnimationFrame(() => el?.focus())
}

function startCreating(parentId: string, collectionId: string, kind: NodeKind, method = 'GET') {
  creating.value = { parentId, collectionId, kind, method }
  creatingName.value = kind === NodeKind.NodeFolder ? 'Новая папка' : 'Новый запрос'
  creatingInvalid.value = false
  if (parentId) store.expanded[parentId] = true
  nextTick(() => {
    focusNextFrame(creatingInput.value)
    creatingInput.value?.select()
  })
}

async function commitCreating() {
  const pending = creating.value
  if (!pending) return
  const name = creatingName.value.trim()
  if (!name) {
    creatingInvalid.value = true
    creatingInput.value?.focus()
    return
  }
  creating.value = null
  creatingInvalid.value = false
  await store.createNode(pending.collectionId, pending.parentId, pending.kind, name, pending.method)
}

function cancelCreating() {
  creating.value = null
  creatingInvalid.value = false
}

function onCreatingKeydown(e: KeyboardEvent) {
  e.stopPropagation()
  if (e.key === 'Enter') {
    e.preventDefault()
    void commitCreating()
  } else if (e.key === 'Escape') {
    e.preventDefault()
    cancelCreating()
  }
}

// What a new row goes inside: the row that was right-clicked — or the collection's own root, which
// is an empty parent rather than the collection's id. A folder holds its children; a collection is
// not its own child.
function parentFor(row: Row): string {
  return row.kind === 'collection' ? '' : row.id
}

const METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE']

function nextMethod() {
  const pending = creating.value
  if (!pending) return
  pending.method = METHODS[(METHODS.indexOf(pending.method) + 1) % METHODS.length]
}

async function addCollection() {
  await leave()
  await store.createCollection(nextCollectionName())
}

// A collection made anywhere — the panel's «+» or the onboarding — is named here, in the row that
// just appeared, which is the gesture the environments sheet has always used.
watch(
  () => store.pendingRename,
  (id) => {
    if (!id) return
    store.pendingRename = null
    const row = visible.value.find((r) => r.id === id)
    if (row) startRename(row)
  },
  { flush: 'post' }
)

// «Новая коллекция», «Новая коллекция 2» … — a default the user is expected to type over, and one
// that does not silently become a second collection of the same name.
function nextCollectionName(): string {
  const taken = new Set(store.tree.map((c) => c.name))
  if (!taken.has('Новая коллекция')) return 'Новая коллекция'
  for (let n = 2; ; n += 1) {
    const name = `Новая коллекция ${n}`
    if (!taken.has(name)) return name
  }
}

// ---- deleting ------------------------------------------------------------

const confirming = ref<Row | null>(null)

async function askRemove(row: Row) {
  // A collection or a folder holding more than one request takes everything inside with it, which is
  // worth a question; an empty folder or a lone request is not.
  if (row.kind !== 'request' && row.count > 1) {
    confirming.value = row
    return
  }
  await remove(row)
}

async function remove(row: Row) {
  confirming.value = null
  await store.remove(row.id)
}

// ---- the unsaved card ----------------------------------------------------

// A card with edits does not disappear quietly: the store raises the alert and answers when it has
// been answered, and the click that asked waits for it. The alert is drawn once, by the window,
// because the rail asks the same question and a card does not have to be left through the tree.
async function leave(): Promise<boolean> {
  return store.askUnsaved()
}

defineExpose({ cancelTop })

function cancelTop(): boolean {
  if (creating.value) {
    cancelCreating()
    return true
  }
  if (renamingId.value) {
    cancelRename()
    return true
  }
  if (confirming.value) {
    confirming.value = null
    return true
  }
  return false
}
</script>

<template>
  <div class="tree-panel">
    <div class="panel-head">
      <span class="panel-title">Коллекции</span>
      <IconButton variant="bare" size="sm" hint="Новая коллекция" @click="addCollection">
        <Icon name="plus" :size="14" />
      </IconButton>
    </div>

    <div class="tree-scroll">
      <ContextMenu v-for="row in visible" :key="row.id">
        <ContextMenuTrigger as-child>
          <div
            class="row"
            :class="{
              'row-collection': row.kind === 'collection',
              active: row.id === store.selectedId,
            }"
            :style="{ paddingLeft: indentDepth(row.depth) }"
            role="button"
            tabindex="0"
            @click="pick(row)"
            @keydown.enter="pick(row)"
            @dblclick="startRename(row)"
          >
            <span
              v-if="row.expandable"
              class="caret"
              :class="{ open: row.expanded }"
              @click.stop="store.toggleExpand(row.id)"
            >
              <Icon name="chevron-right" :size="10" />
            </span>
            <span v-else class="caret-space"></span>

            <span v-if="row.kind === 'request'" class="row-method mono">{{ row.method }}</span>

            <input
              v-if="renamingId === row.id"
              :ref="setNameInput"
              v-model="nameDraft"
              class="row-rename"
              :class="{ invalid: renameInvalid }"
              spellcheck="false"
              @click.stop
              @dblclick.stop
              @keydown="onRenameKeydown"
              @blur="commitRenameFrom(row.id)"
            />
            <span v-else class="row-name" :title="row.name">{{ row.name }}</span>

            <span v-if="row.kind !== 'request'" class="row-count mono">{{ row.count }}</span>
          </div>
        </ContextMenuTrigger>

        <ContextMenuContent>
          <template v-if="row.kind !== 'request'">
            <ContextMenuItem @select="startCreating(parentFor(row), row.collectionId, NodeKind.NodeFolder)">
              Новая папка
            </ContextMenuItem>
            <ContextMenuItem @select="startCreating(parentFor(row), row.collectionId, NodeKind.NodeRequest)">
              Новый запрос
            </ContextMenuItem>
            <ContextMenuSeparator />
          </template>
          <ContextMenuItem @select="startRename(row)">Переименовать</ContextMenuItem>
          <ContextMenuItem @select="store.duplicate(row.id)">Дублировать</ContextMenuItem>
          <ContextMenuItem @select="exportNode(row)">
            {{ row.kind === 'request' ? 'Экспорт запроса' : 'Экспорт' }}
          </ContextMenuItem>
          <ContextMenuSeparator />
          <ContextMenuItem class="danger" @select="askRemove(row)">Удалить</ContextMenuItem>
        </ContextMenuContent>
      </ContextMenu>

      <div v-if="creating" class="row row-creating" :style="{ paddingLeft: creatingIndent(creating.parentId) }">
        <span class="caret-space"></span>
        <button
          v-if="creating.kind === 'request'"
          class="row-method mono method-chip"
          title="Сменить метод"
          @click="nextMethod"
        >
          {{ creating.method }}
        </button>
        <input
          :ref="setCreatingInput"
          v-model="creatingName"
          class="row-rename"
          :class="{ invalid: creatingInvalid }"
          spellcheck="false"
          @keydown="onCreatingKeydown"
          @blur="commitCreating"
        />
      </div>

      <div v-if="visible.length === 0 && store.tree.length > 0" class="no-results">Ничего не найдено</div>
    </div>

    <div class="panel-filter-dock">
      <div class="panel-filter-fade"></div>
      <div class="panel-filter">
        <Input v-model="query" size="sm" class="w-full" placeholder="Поиск по коллекции…" spellcheck="false" />
      </div>
      <div class="panel-filter-backdrop"></div>
    </div>

    <DeleteNodeDialog
      v-if="confirming"
      :open="true"
      :name="confirming?.name ?? ''"
      :count="confirming?.count ?? 0"
      @cancel="confirming = null"
      @confirm="confirming && remove(confirming)"
    />
  </div>
</template>

<style scoped>
@reference "../../style.css";

.tree-panel {
  @apply relative flex flex-col h-full min-h-0 bg-bg-panel border-r border-border;
}

.panel-head {
  @apply flex items-center justify-between h-10 px-1 pl-3.5 border-b border-border;
}

.panel-title {
  @apply text-sm font-semibold text-text-secondary;
}

.tree-scroll {
  @apply flex-1 min-h-0 overflow-y-auto pt-1 pb-23;
}

.row {
  @apply flex items-center gap-1.5 h-[26px] pr-2 rounded-md text-text text-[13px] cursor-pointer;
}

.row:hover {
  @apply bg-bg-hover;
}

.row.active {
  @apply bg-accent-soft text-accent;
}

.row-collection {
  @apply font-semibold;
}

.caret {
  @apply flex-none inline-flex items-center justify-center w-3.5 text-text-tertiary transition-transform duration-150;
}

.caret.open {
  transform: rotate(90deg);
}

.caret-space {
  @apply flex-none w-3.5;
}

.row-method {
  @apply flex-none text-[10px] font-semibold text-text-tertiary;
  min-width: 40px;
}

.method-chip {
  @apply bg-transparent border border-border rounded-sm py-px px-1 text-text-secondary cursor-pointer;
}

.row-name {
  @apply flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap;
}

.row-count {
  @apply flex-none text-xs text-text-tertiary;
}

.row-rename {
  @apply flex-1 min-w-0 h-[22px] box-border text-[13px] outline-none;
  padding: 0 6px;
  border-radius: 5px;
  border: 1px solid var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
  background: var(--bg-panel);
  color: var(--text);
}

.row-rename.invalid {
  border-color: color-mix(in srgb, var(--red) 45%, transparent);
  box-shadow: 0 0 0 3px var(--red-soft);
}

.row-creating {
  @apply cursor-default;
}

.no-results {
  @apply pt-4 px-4 text-center text-text-tertiary text-xs;
}

.panel-filter-dock {
  @apply absolute left-0 right-0 bottom-0 flex flex-col pointer-events-none;
}

.panel-filter-fade {
  @apply h-8;
  background: linear-gradient(
    to bottom,
    color-mix(in srgb, var(--bg-panel) 0%, transparent) 0%,
    color-mix(in srgb, var(--bg-panel) 30%, transparent) 40%,
    color-mix(in srgb, var(--bg-panel) 70%, transparent) 70%,
    var(--bg-panel) 100%
  );
}

.panel-filter {
  @apply pointer-events-auto flex items-center py-1.5 px-3 bg-bg-panel;
}

.panel-filter-backdrop {
  @apply h-2 bg-bg-panel;
}
</style>
