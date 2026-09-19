<script setup lang="ts">
import { computed, nextTick, ref, watch, type ComponentPublicInstance } from 'vue'
import Icon from '../ui/Icon.vue'
import { IconButton } from '../ui/button'
import PanelFilter from '../ui/PanelFilter.vue'
import {
  ContextMenu,
  ContextMenuTrigger,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
} from '../ui/context-menu'
import DeleteNodeDialog from './DeleteNodeDialog.vue'
import { useListKeys } from '../../composables/useListKeys'
import { methodInkClass, shortMethod } from '../../lib/format'
import { useTreeDrag, type DropTarget } from '../../composables/useTreeDrag'
import { useCollectionsStore } from '../../stores/collections'
import { childrenOf, filterTree, findCollection, requestCount, trailOf } from '../../lib/collectionTree'
import { useToast } from '../../composables/useToast'
import { usePlatform } from '../../composables/usePlatform'
import { useMessages } from '../../i18n'
import type { Collection } from '../../../bindings/json-inspector/internal/domain'

const store = useCollectionsStore()
const { t } = useMessages()
const toast = useToast()
// The keys the menu promises and the panel answers to are built here, once, so a hint cannot drift
// from the binding it names.
const { shortcut, keyLabel, chord } = usePlatform()

// One request, one file: the menu exports what it was opened on, and a collection is exported from its
// own overview.
async function exportNode(row: Row) {
  const written = await store.exportFile(row.id)
  if (written) toast.show(t('collections.savedToFileNamed', { name: row.name }))
}

// The indent the design gives a level, as the row's own left padding: the first starts where the panel
// does — 16px, the same edge the head's title stands on — and each level below adds 14. A collection
// may hold a collection, and one of those another, so the ladder keeps climbing rather than piling the
// deepest rows on one column.
const INDENT = 16
const STEP = 14

// A row of the tree, flattened: the drawing walks a list, and the nesting is what the indent says.
// Expansion is applied here rather than by the tree, so the filter can open a path to a match
// without moving anything the user opened.
interface Row {
  id: string
  kind: 'collection' | 'request'
  name: string
  method: string
  count: number
  depth: number
  expandable: boolean
  expanded: boolean
  // The collection the row sits in. A row is dropped into a level, and this is which one; a row at the
  // top of the tree sits in none of them, which is the empty id Go reads as "the top level". A
  // collection's own row is not inside itself: what its id names is the level it holds.
  collectionId: string
  // Where the row stands among the rows of that level, requests and collections counted together — the
  // index a drop names.
  position: number
  // A collection that follows another one carries the line the design separates them with.
  divider: boolean
}

const query = computed({
  get: () => store.filter,
  set: (value: string) => store.setFilter(value),
})

const searching = computed(() => store.filter.trim().length > 0)

const visible = computed<Row[]>(() => flatten(filterTree(store.tree, store.filter), false))

// What the arrows walk: the same tree with every level open. A walk that stopped at each closed level
// would be a walk the user has to open the tree for — and reaching a folder without touching the
// chevron is what the arrows are for. The row the walk lands on is brought into view by opening what
// is above it, so the selection is always a row the user can see.
const walkable = computed<Row[]>(() => flatten(filterTree(store.tree, store.filter), true))

function flatten(tree: Collection[], allOpen: boolean): Row[] {
  const rows: Row[] = []
  for (const collection of tree) {
    // Top-level collections stand on the workspace's own level, which has no id: an empty one is what a
    // drop beside them names, and Go reads it as the top level.
    rows.push(...level(collection, '', 0, allOpen, rows.length === 0 ? false : true))
  }
  return rows
}

// level draws one collection and everything inside it: its own request rows and the collections nested
// in it, in the order Go keeps them in — one number line, requests and collections alternating by
// position. holderId is the level the collection itself is a row of: the empty one at the top of the
// tree, and the collection holding it everywhere below.
function level(collection: Collection, holderId: string, depth: number, allOpen: boolean, divider: boolean): Row[] {
  const expanded = allOpen || searching.value || store.expanded[collection.id]
  const children = childrenOf(collection)

  const rows: Row[] = [
    {
      id: collection.id,
      kind: 'collection',
      name: collection.name,
      method: '',
      count: requestCount(collection),
      depth,
      expandable: children.length > 0,
      expanded: !!expanded,
      collectionId: holderId,
      position: collection.position,
      // The design draws a hairline above every collection but the first of a level: the trees of two
      // collections are two things, and without the line a name under a subtree reads as its child.
      divider,
    },
  ]
  if (!expanded) return rows

  // What a row is numbered by is its place in the level, not the number Go stores: the two agree
  // after every move, and a drop has to name a place among the rows the window drew.
  children.forEach((child, at) => {
    if (child.kind === 'collection') {
      rows.push(...level(child.collection, collection.id, depth + 1, allOpen, false))
      return
    }
    rows.push({
      id: child.node.id,
      kind: 'request',
      name: child.node.name,
      method: child.node.method ?? 'GET',
      count: 0,
      depth: depth + 1,
      expandable: false,
      expanded: false,
      collectionId: collection.id,
      position: at,
      divider: false,
    })
  })
  return rows
}

// The indent a row is drawn at: one step per level, from the edge the panel's own content starts on.
function indentDepth(depth: number): string {
  return `${INDENT + depth * STEP}px`
}

// A click selects and opens: a request as a card, a collection as its overview. What is inside a
// collapsed row is reached by its chevron, which is the gesture the tree has always had.
async function pick(row: Row) {
  if (!(await leave())) return
  await store.select(row.id)
}

// ---- carrying a row ------------------------------------------------------

const treeEl = ref<HTMLElement | null>(null)

// The element a row is drawn in. The drag is the only thing that needs a row by id rather than by
// index, so it is found in the drawing instead of kept in a map that would have to be kept in step
// with it.
function elementOf(id: string): HTMLElement | null {
  return treeEl.value?.querySelector<HTMLElement>(`[data-id="${id}"]`) ?? null
}

// Whether a collection would take a row dropped into it. A collection inside itself, or inside one of
// its own, is a ring — and Go refuses the same thing, so the window does not offer it as a place.
function acceptsDrag(dragged: Row, collectionId: string): boolean {
  if (dragged.kind === 'request') return true
  if (dragged.id === collectionId) return false
  const from = findCollection(store.tree, dragged.id)
  return !from || findCollection(from.children ?? [], collectionId) === null
}

// Where a drop lands, in the coordinates Go is told about: a collection, and a place among the rows of
// its level. A row dropped into a collection joins the end of it; dropped above or below another row
// it takes that row's place, counted in the level as it looks now — the row being carried included.
function dropOn(dragged: Row, target: DropTarget) {
  const place = placeOf(dragged, target)
  if (!place) return
  if (dragged.kind === 'collection') {
    void store.moveCollection(dragged.id, place.collectionId, place.position)
    return
  }
  void store.moveNode(dragged.id, place.collectionId, place.position)
}

function placeOf(dragged: Row, target: DropTarget): { collectionId: string; position: number } | null {
  if (target.zone === 'inside') {
    const collection = findCollection(store.tree, target.id)
    if (!collection) return null
    return { collectionId: collection.id, position: childrenOf(collection).length }
  }

  const at = visible.value.find((row) => row.id === target.id)
  if (!at) return null

  // A request has no level of its own beside a top-level collection: the empty id is the top of the
  // tree, and a request lives in a collection and nowhere else. There the line above the row is the head
  // of that collection and the line below is its tail — which is where a request dropped there lands.
  if (at.collectionId === '' && dragged.kind === 'request') {
    const root = findCollection(store.tree, at.id)
    if (!root) return null
    return { collectionId: root.id, position: target.zone === 'before' ? 0 : childrenOf(root).length }
  }

  // Otherwise the row takes a place in the level the row it was dropped beside stands on — for a
  // collection that is the level holding it, not the one it holds — and at the top of the tree that
  // level has no id at all, which is how Go is told the row belongs there.
  return { collectionId: at.collectionId, position: at.position + (target.zone === 'after' ? 1 : 0) }
}

const { drag, press, swallowClick } = useTreeDrag({
  rows: () => visible.value,
  elementOf,
  accepts: acceptsDrag,
  open: (id) => {
    store.expanded[id] = true
  },
  move: dropOn,
})

// The click the browser sends after a carry would select the row the user just moved — and the row it
// landed on is what the gesture was about.
function onClickCapture(e: MouseEvent) {
  if (!swallowClick()) return
  e.stopPropagation()
  e.preventDefault()
}

function dropClass(row: Row): Record<string, boolean> {
  const target = drag.value?.target
  return {
    'drop-into': target?.id === row.id && target.zone === 'inside',
    'drop-before': target?.id === row.id && target.zone === 'before',
    'drop-after': target?.id === row.id && target.zone === 'after',
  }
}

// Opens the levels above a row, so that a row the walk reached is a row on screen. The row itself is
// left as it was: the walk moved the selection, and what is open is the user's own answer.
function reveal(row: Row) {
  const trail = trailOf(store.tree, row.id)
  // A collection at the top is the top of its own tree: there is nothing above it to open, and opening
  // it would be opening the row the walk stands on.
  if (!trail) return
  for (const ancestor of trail.ancestors) store.expanded[ancestor.id] = true
}

// The arrows of a tree: up and down walk the tree, and left and right are the opening and the
// closing — a closed row opens, an open one closes, and left on either steps out to what holds it.
// That is the gesture the chevron makes with a mouse, on the key a file list has always answered to.
useListKeys({
  ids: () => walkable.value.map((row) => row.id),
  current: () => store.selectedId,
  move: (id) => {
    const row = walkable.value.find((r) => r.id === id)
    if (!row) return
    reveal(row)
    void pick(row)
  },
  selected: '.tree-panel .panel-row.active',
  onSideKey: (key, id) => {
    const rows = visible.value
    const at = rows.findIndex((row) => row.id === id)
    if (at < 0) return false
    const row = rows[at]

    if (key === 'ArrowRight') {
      if (!row.expandable) return false
      if (row.expanded) {
        // Already open: what is inside it is the next row, drawn one level deeper.
        const child = rows[at + 1]
        if (child && child.depth > row.depth) void pick(child)
        return true
      }
      store.toggleExpand(row.id)
      return true
    }

    if (row.expandable && row.expanded) {
      store.toggleExpand(row.id)
      return true
    }
    // A request, or a row that is already closed: left goes out to the level that holds it.
    for (let i = at - 1; i >= 0; i--) {
      if (rows[i].depth < row.depth) {
        void pick(rows[i])
        break
      }
    }
    return true
  },
})

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
const creating = ref<{ collectionId: string } | null>(null)
const creatingName = ref('')
const creatingInput = ref<HTMLInputElement | null>(null)
const creatingInvalid = ref(false)

// Where that row is drawn: at the end of the level the request is about to join, which is where it
// will actually be. "After the collection's own row" would be a different place whenever the
// collection has rows of its own — and it is opened as the naming starts, so it always shows them.
const creatingAt = computed<{ after: string; depth: number } | null>(() => {
  const target = creating.value?.collectionId
  if (!target) return null

  const rows = visible.value
  const at = rows.findIndex((row) => row.id === target)
  if (at < 0) return null

  let end = at
  while (end + 1 < rows.length && rows[end + 1].depth > rows[at].depth) end += 1
  return { after: rows[end].id, depth: rows[at].depth + 1 }
})

function setCreatingInput(el: Element | ComponentPublicInstance | null) {
  creatingInput.value = (el as HTMLInputElement | null) ?? null
}

// A field that appears where a menu item was clicked cannot be focused in the same tick: the menu is
// still closing, and whatever it does with focus on the way out lands after this. The next frame is
// the first moment the caret stays where it was put.
function focusNextFrame(el: HTMLInputElement | null) {
  requestAnimationFrame(() => el?.focus())
}

// A request is created in the collection it will live in — the one that was right-clicked. A
// collection is not made here: the «+» in the head makes one at the top, and a drop is what puts it
// inside another.
//
// What is typed here is the name and nothing else: the row carries no method, and the request is made
// as GET — the method is chosen in the card the row opens, where the address is filled in too.
// The level a row's new request would join: a collection's own row is that collection, while a
// request's row names the level it sits in. The menu, the key and the new-folder gesture all ask it.
function levelOf(row: Row): string {
  return row.kind === 'collection' ? row.id : row.collectionId
}

function startCreating(collectionId: string) {
  creating.value = { collectionId }
  creatingName.value = t('collections.newRequest')
  creatingInvalid.value = false
  store.expanded[collectionId] = true
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
  await store.createNode(pending.collectionId, name, 'GET')
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

// A folder is a collection inside one, so the panel's «+» is this same call without a level: the one
// that exists where the user is looking is the menu's.
async function addCollection(parentId = '') {
  await leave()
  await store.createCollection(nextCollectionName(), parentId)
}

// Running a level is the overview's own gesture, done from the tree: the folder opens first, because
// that page is where the rows of the run appear — a run watched only through the status bar's counter
// is a run nobody can follow.
async function runNode(row: Row) {
  if (row.id !== store.selectedId) await store.select(row.id)
  await store.run(store.runNodeId, row.name)
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

// A row the window reached from somewhere other than this tree — a row of a run, a hit in the
// palette — is a row whose parents may be closed and which may be scrolled out of sight. Opening
// them and bringing it into view is what makes the selection a place rather than a value: from the
// run's table the whole point of the click is to arrive at the request. Running on mount as well,
// because the tree is drawn after the section is switched to.
watch(
  () => store.selectedId,
  async (id) => {
    if (!id) return
    if (!visible.value.some((row) => row.id === id)) {
      const trail = trailOf(store.tree, id)
      if (!trail) return
      for (const ancestor of trail.ancestors) store.expanded[ancestor.id] = true
      await nextTick()
    }
    elementOf(id)?.scrollIntoView({ block: 'nearest' })
  },
  { immediate: true, flush: 'post' }
)

// The key that makes a request names the level it goes in and leaves the naming here, where the row
// is: the box opens on the place the row will appear rather than making one with a name nobody chose.
watch(
  () => store.pendingCreate,
  (collectionId) => {
    if (!collectionId) return
    store.pendingCreate = null
    startCreating(collectionId)
  },
  { flush: 'post' }
)

// «Новая коллекция», «Новая коллекция 2» … — a default the user is expected to type over, and one
// that does not silently become a second collection of the same name. The name is written in the
// language it was made in and stays that way: from here on it is the user's own text, not a label.
function nextCollectionName(): string {
  const taken = new Set(store.tree.map((c) => c.name))
  const base = t('collections.newCollection')
  if (!taken.has(base)) return base
  for (let n = 2; ; n += 1) {
    const name = t('collections.newCollectionN', { n })
    if (!taken.has(name)) return name
  }
}

// ---- deleting ------------------------------------------------------------

const confirming = ref<Row | null>(null)

async function askRemove(row: Row) {
  // A collection holding more than one request takes everything inside with it, which is worth a
  // question; an empty collection or a lone request is not.
  if (row.kind === 'collection' && row.count > 1) {
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
      <span class="panel-title">{{ t('collections.title') }}</span>
      <IconButton variant="bare" class="panel-add" :hint="t('collections.newCollection')" @click="addCollection()">
        <Icon name="plus" :size="16" />
      </IconButton>
    </div>

    <div ref="treeEl" class="tree-scroll" @click.capture="onClickCapture">
      <template v-for="row in visible" :key="row.id">
        <ContextMenu>
          <ContextMenuTrigger as-child>
            <div
              class="panel-row"
              :data-id="row.id"
              :class="{
                'row-collection': row.kind === 'collection',
                'row-divider': row.divider,
                active: row.id === store.selectedId,
                'row-carried': drag?.row.id === row.id,
                ...dropClass(row),
              }"
              :style="{ paddingLeft: indentDepth(row.depth) }"
              role="button"
              tabindex="0"
              @pointerdown="press($event, row)"
              @click="pick(row)"
              @keydown.enter="pick(row)"
              @dblclick="startRename(row)"
            >
              <span
                v-if="row.expandable"
                class="caret"
                :class="{ open: row.expanded }"
                @pointerdown.stop
                @click.stop="store.toggleExpand(row.id)"
              >
                <Icon name="chevron-right" :size="11" />
              </span>
              <!-- Only a collection holds a level, so only a collection keeps room for the chevron: a
                   request starts at its own indent, which is what the design draws and what keeps the
                   two kinds of row apart at a glance. -->
              <span v-else-if="row.kind === 'collection'" class="caret-space"></span>

              <span
                v-if="row.kind === 'collection'"
                class="row-icon"
                :class="{ 'row-icon-top': row.depth === 0 }"
              >
                <Icon name="folder" :size="14" />
              </span>

              <!-- A request's verb is drawn as every other list draws it: the shared method cell,
                   coloured by the verb and cut to the column, so the tree's rows and the history's
                   read as one list. -->
              <span
                v-if="row.kind === 'request'"
                class="panel-method"
                :class="methodInkClass(row.method)"
              >{{ shortMethod(row.method) }}</span>

              <input
                v-if="renamingId === row.id"
                :ref="setNameInput"
                v-model="nameDraft"
                class="row-rename"
                :class="{ invalid: renameInvalid }"
                spellcheck="false"
                @pointerdown.stop
                @click.stop
                @dblclick.stop
                @keydown="onRenameKeydown"
                @blur="commitRenameFrom(row.id)"
              />
              <span v-else class="panel-address" :title="row.name">{{ row.name }}</span>

              <span v-if="row.kind === 'collection'" class="row-count mono">{{ row.count }}</span>
            </div>
          </ContextMenuTrigger>

          <ContextMenuContent class="tree-menu">
            <!-- What the menu is about, named above it: a menu opened on the wrong row says so before
                 anything is clicked. -->
            <div class="menu-title">{{ row.name }}</div>
            <template v-if="row.kind === 'collection'">
              <ContextMenuItem @select="startCreating(levelOf(row))">
                <span class="menu-label-text">{{ t('collections.newRequest') }}</span>
                <span class="menu-key">{{ shortcut('N') }}</span>
              </ContextMenuItem>
              <ContextMenuItem @select="addCollection(row.id)">
                <span class="menu-label-text">{{ t('collections.newFolder') }}</span>
              </ContextMenuItem>
              <ContextMenuItem @select="runNode(row)">
                <span class="menu-label-text">{{ t('collections.runFolder') }}</span>
                <span v-if="chord('R')" class="menu-key">{{ shortcut('R') }}</span>
              </ContextMenuItem>
              <ContextMenuSeparator />
            </template>
            <template v-else>
              <ContextMenuItem @select="pick(row)">
                <span class="menu-label-text">{{ t('collections.openRequest') }}</span>
                <span class="menu-key">{{ keyLabel('enter') }}</span>
              </ContextMenuItem>
            </template>
            <ContextMenuItem @select="startRename(row)">
              <span class="menu-label-text">{{ t('collections.rename') }}</span>
            </ContextMenuItem>
            <ContextMenuItem @select="store.duplicate(row.id)">
              <span class="menu-label-text">{{ t('collections.duplicate') }}</span>
              <span class="menu-key">{{ shortcut('D') }}</span>
            </ContextMenuItem>
            <ContextMenuItem @select="exportNode(row)">
              <span class="menu-label-text">
                {{ row.kind === 'request' ? t('collections.exportRequest') : t('collections.export') }}
              </span>
            </ContextMenuItem>
            <ContextMenuSeparator />
            <ContextMenuItem class="danger" @select="askRemove(row)">
              <span class="menu-label-text">{{ t('common.delete') }}</span>
            </ContextMenuItem>
          </ContextMenuContent>
        </ContextMenu>

        <div
          v-if="creatingAt && creatingAt.after === row.id"
          class="panel-row row-creating"
          :style="{ paddingLeft: indentDepth(creatingAt.depth) }"
        >
          <!-- The verb's own room: the name being typed stands where the name will stand. -->
          <span class="panel-method-space"></span>
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
      </template>

      <div v-if="visible.length === 0 && store.tree.length > 0" class="no-results">{{ t('common.nothingFound') }}</div>
    </div>

    <!-- What the hand is carrying: the row's own words on the material the popovers use, following the
         pointer so that the gesture says what it holds. -->
    <Teleport to="body">
      <div v-if="drag" class="drag-ghost" :style="{ left: `${drag.x + 12}px`, top: `${drag.y + 8}px` }">
        <span
          v-if="drag.row.kind === 'request'"
          class="panel-method"
          :class="methodInkClass(drag.row.method)"
        >{{ shortMethod(drag.row.method) }}</span>
        <span class="panel-address">{{ drag.row.name }}</span>
      </div>
    </Teleport>

    <PanelFilter v-model="query" :placeholder="t('collections.treeSearch')" />

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
  /* The seam against the content belongs to the panel's frame, which knows which edge it is on, and
     so does the fill: the panel stands on the sidebar's glass. The head, the rows and the foot are
     the shared panel shapes in style.css; what is here is only what a tree adds to them. */
  @apply relative flex flex-col h-full min-h-0;
}

/* Against the shared icon button's own size, which is the size the rest of the window uses. Deep
   because a hinted icon button is drawn inside a tooltip: the element is the tooltip's, and the
   panel's own scope never reaches it. */
.panel-head :deep(.panel-add) {
  @apply w-[28px] h-[28px] rounded-[7px] text-accent;
}

.panel-head :deep(.panel-add:hover:not(:disabled)) {
  @apply bg-accent-soft text-accent;
}

/* No gap and no inset of the container's: the row carries its own padding, and every level is that
   padding plus its own indent, so a row's hover fill reaches the edge the panel starts from. */
.tree-scroll {
  @apply flex-1 min-h-0 overflow-y-auto flex flex-col;
}

/* The tree is the one panel whose open row inks the whole label with the accent: a name is what a row
   of a tree is, and the tint alone would leave the name reading like any other. */
.panel-row.active {
  color: var(--accent);
}

.row-collection {
  @apply font-semibold;
}

/* A collection after the first is a new tree, and the line above it says so. The line is drawn rather
   than bordered: a border on a row would bend around its own edges, and that curve shows as a smudge
   above the fill of a row that is hovered or selected. */
.row-divider::before {
  content: '';
  @apply absolute left-0 right-0 h-px;
  top: 0;
  background: var(--border);
}

.row-icon {
  @apply flex-none text-text-secondary;
}

/* The folder is the accent for a collection at the top of the tree and the grey of the furniture for
   one inside another: the top level is what the panel is a list of, and the levels under it are its
   contents. */
.row-icon-top {
  @apply text-accent;
}

.panel-row.active .row-icon {
  @apply text-accent;
}

.caret {
  @apply flex-none inline-flex items-center justify-center w-[11px] text-text-tertiary transition-transform duration-150;
}

.caret.open {
  transform: rotate(90deg);
}

.caret-space {
  @apply flex-none w-[11px];
}

/* The verb and the name are the shared cells of the three lists — the method's column, the name that
   fills the rest — so they are not repeated here. What a tree adds is only what a tree has. */
.row-count {
  @apply flex-none text-[11px] text-text-tertiary tabular-nums;
}

.row-rename {
  @apply flex-1 min-w-0 h-[24px] box-border text-[13px] outline-none;
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

/* The row being carried stays where it is and is drawn as what it will leave behind: the drop target
   is what the eye should be reading, and a row that jumped about under the pointer would be the
   loudest thing on screen. */
.row-carried {
  @apply opacity-40;
}

/* Where the row would land. A line above or below a row is a place in the level; the fill is a
   collection that would take it. The two are the accent the selection already uses rather than colours
   of their own: a drop is a place, and the tree has one colour for "here". */
.drop-before::after,
.drop-after::after {
  content: '';
  @apply absolute left-0 right-0 h-0.5 rounded-full;
  background: var(--accent);
}

.drop-before::after {
  top: -1px;
}

.drop-after::after {
  bottom: -1px;
}

.drop-into {
  @apply bg-accent-soft;
  box-shadow: inset 0 0 0 1px var(--accent);
}

.drag-ghost {
  @apply fixed z-50 flex items-center gap-2 py-1.5 px-2.5 rounded-lg text-text text-[13px] pointer-events-none;
  background: var(--glass-overlay);
  backdrop-filter: var(--blur-overlay);
  box-shadow: 0 8px 24px rgb(0 0 0 / 0.18);
}

</style>
