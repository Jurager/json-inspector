<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import Icon from '../ui/Icon.vue'
import { Button } from '../ui/button'
import { Popover, PopoverAnchor, PopoverContent } from '../ui/popover'
import { useCollectionsStore } from '../../stores/collections'
import { usePlatform } from '../../composables/usePlatform'
import { useToast } from '../../composables/useToast'
import { useMessages } from '../../i18n'
import { collectionPlaces } from '../../lib/collectionPlaces'

const { t } = useMessages()

// Where a request composed in the command line goes when it is saved, in two steps: the place, then
// the name. The order is the order the questions matter in — a request nobody can find is a request
// saved nowhere, and what it is called is the second thing, and one the address has already answered
// for most requests.
//
// What the first step offers is every level of the tree — a collection and a folder are the same
// thing at different depths — and, for a name nothing answers to, the collection that name would
// make. Whether the row already exists is the only question the search asks, which is what lets one
// field serve as both the filter and the name of a new collection.
const props = defineProps<{ open: boolean; url: string; defaultName: string }>()
// `saved` says the request is in the tree now. What the line makes of that is the line's to say —
// this knows about collections and nothing about a draft.
const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  (e: 'saved'): void
}>()

const store = useCollectionsStore()
const toast = useToast()
const { shortcut } = usePlatform()

// One row of the list: an existing level of the tree, or the collection a typed name would make. The
// key is what the save is addressed by — an id, or the name still to be created.
interface Target {
  key: string
  name: string
  meta: string
  created: boolean
}

// Every level of the tree, deepest ones included, each named by its whole path: a folder called
// «Articles» says nothing about which collection holds it, and this list is flat.
const levels = computed<Target[]>(() =>
  collectionPlaces(store.tree).map((place) => ({
    key: place.id,
    name: place.path.join(' / '),
    meta: t('collections.savePlace', {
      kind: t(place.path.length > 1 ? 'collections.folderTag' : 'collections.aCollection'),
      requests: t('counts.requests', place.requests),
    }),
    created: false,
  }))
)

const query = ref('')
const chosen = ref('')
const name = ref('')
const step = ref<'pick' | 'confirm'>('pick')
const saving = ref(false)

// What the search matched, plus the one row that is not in the tree: a name nothing answers to is a
// collection to make. It stands first, because it is what the typed text is about.
const targets = computed<Target[]>(() => {
  const typed = query.value.trim()
  const needle = typed.toLowerCase()
  const found = levels.value.filter((level) => !needle || level.name.toLowerCase().includes(needle))
  const names = needle && !levels.value.some((level) => level.name.toLowerCase() === needle)

  if (!names) return found
  return [
    {
      key: `new:${typed}`,
      name: t('collections.saveCreate', { name: typed }),
      meta: t('collections.newCollection'),
      created: true,
    },
    ...found,
  ]
})

// The row the second step is about. It is read off the key rather than kept as a row, so a tree that
// changed under the panel cannot leave the step naming a place that is not in the list.
const target = computed(() => targets.value.find((row) => row.key === chosen.value) ?? null)

const targetName = computed(() => {
  const place = target.value
  if (!place) return ''
  return place.created ? t('collections.saveTargetNew', { name: place.key.slice(4) }) : place.name
})

// Both steps answer the same question about the request: is there anything to save. The address is
// asked for as well because it can be cleared while the panel is open — the line behind it is still
// the user's.
const canSave = computed(() => !!target.value && props.url.trim().length > 0)

// The panel opens on the first step, over an empty search and with the address's own suggestion in
// the name: what was typed into either of them belongs to the request that was on screen then.
watch(
  () => props.open,
  (open) => {
    if (!open) return
    query.value = ''
    name.value = props.defaultName
    step.value = 'pick'
    chosen.value = store.collectionId || levels.value[0]?.key || ''
    nextTick(() => searchInput.value?.focus())
  }
)

// A chosen row that the search has just filtered out is not on screen any more: the list's own first
// row is what is really under the cursor. It is not reached while the second step is up — that step
// draws no search, and the row it is about is the one that was picked.
watch(targets, (rows) => {
  if (step.value === 'pick' && !rows.some((row) => row.key === chosen.value)) {
    chosen.value = rows[0]?.key ?? ''
  }
})

const searchInput = ref<HTMLInputElement | null>(null)
const nameInput = ref<HTMLInputElement | null>(null)

// Picking a place is what opens the second step: the row is a place to save into, and what is left to
// answer is what the request is called.
function pick(row: Target) {
  chosen.value = row.key
  step.value = 'confirm'
  // Focused and not selected: the field opens on the address's own suggestion, and the part of it a
  // person reads first is the front — what is selected is scrolled to its end, which on a long
  // address is the query.
  nextTick(() => nameInput.value?.focus())
}

function back() {
  step.value = 'pick'
  nextTick(() => searchInput.value?.focus())
}

function move(step2: number) {
  const rows = targets.value
  if (rows.length === 0) return
  const at = rows.findIndex((row) => row.key === chosen.value)
  chosen.value = rows[(at + step2 + rows.length) % rows.length].key
}

// On the first step the arrows walk the list and Enter opens the row under the cursor; on the second
// the field is the window's and Enter is what saves. One handler, because the two steps are one
// control and the keyboard should not have to be told which of them is up.
function onPickKeydown(e: KeyboardEvent) {
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    move(1)
    return
  }
  if (e.key === 'ArrowUp') {
    e.preventDefault()
    move(-1)
    return
  }
  if (e.key === 'Enter') {
    e.preventDefault()
    const row = targets.value.find((r) => r.key === chosen.value)
    if (row) pick(row)
  }
}

// The button this sheet hangs from stands outside its own content, so pressing it reaches reka-ui as
// an outside interaction and closes the sheet — and the button's `click`, running against that
// closed state, opens it again. The button could then never close what it opened, and a press held
// down flickered. Suppressing it for that button hands the whole decision to the button itself.
//
// The class is the one the request bar wraps its save button and this sheet in: the two are one
// control, and the sheet is opened from nowhere else.
//
// The target is an Element and not an HTMLElement: the press usually lands on the icon inside the
// button, and an SVG is not an HTMLElement — a guard that asks for one never fires on it.
function onInteractOutside(e: Event) {
  const target = (e as CustomEvent<{ originalEvent?: Event }>).detail?.originalEvent?.target
  if (target instanceof Element && target.closest('.save-anchor')) e.preventDefault()
}

async function save() {
  const place = target.value
  const called = name.value.trim() || props.defaultName
  if (!place || !canSave.value || saving.value) return
  saving.value = true
  try {
    const done = place.created
      ? await store.saveDraftToNew(place.key.slice(4), called)
      : await store.saveDraft(place.key, called)
    // A refusal has already been said out loud, and the panel stays open on it: what it holds is what
    // the person typed, and closing it would throw that away along with the save.
    if (!done) return
    toast.show(t('collections.savedTo', { name: targetName.value }))
    emit('saved')
    emit('update:open', false)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <Popover :open="props.open" @update:open="(v) => emit('update:open', v)">
    <PopoverAnchor as-child>
      <span class="anchor"></span>
    </PopoverAnchor>

    <PopoverContent align="end" @open-auto-focus.prevent @interact-outside="onInteractOutside">
      <div class="sheet-body">
        <!-- The first step: where. A row is a place, and opening it is what goes on to the name. -->
        <div v-if="step === 'pick'" class="step step-back">
          <div class="head">{{ t('collections.saveRequestTo') }}</div>

          <label class="search">
            <Icon name="search" :size="14" :stroke-width="1.9" />
            <input
              ref="searchInput"
              v-model="query"
              type="text"
              :placeholder="t('collections.saveFindPlace')"
              spellcheck="false"
              @keydown="onPickKeydown"
            />
          </label>

          <div class="places">
            <button
              v-for="row in targets"
              :key="row.key"
              class="place"
              :class="{ active: row.key === chosen }"
              @click="pick(row)"
            >
              <span class="place-mark">
                <Icon v-if="row.key === chosen && row.created" name="plus" :size="14" :stroke-width="2.2" />
                <Icon v-else-if="row.key === chosen" name="check" :size="15" :stroke-width="2.6" />
              </span>
              <span class="place-text">
                <span class="place-name">{{ row.name }}</span>
                <span class="place-meta">{{ row.meta }}</span>
              </span>
            </button>
            <!-- The only way the list is empty: nothing has been made yet. A query always has the row
                 that would make a collection of it, and a name that answers to a level is that level. -->
            <div v-if="targets.length === 0" class="no-places">
              {{ t('collections.createCollectionFirst') }}
            </div>
          </div>
        </div>

        <!-- The second step: what it is called. The place is what the row above says, and the way back
             to it is the button beside that name. -->
        <div v-else class="step step-next">
          <div class="confirm-head">
            <button type="button" class="back" :title="t('collections.saveAnotherPlace')" @click="back">
              <Icon name="chevron-left" :size="15" :stroke-width="2.2" />
            </button>
            <span class="target" :title="targetName">{{ targetName }}</span>
          </div>

          <input
            ref="nameInput"
            v-model="name"
            class="name-input"
            type="text"
            :placeholder="t('collections.saveNamePlaceholder')"
            spellcheck="false"
            @keydown.enter.prevent="save"
          />
          <span class="name-hint">{{ t('collections.saveNameHint') }}</span>

          <div class="foot">
            <span class="foot-state">{{ t('collections.saveShortcutNext', { shortcut: shortcut('S') }) }}</span>
            <Button class="save-btn" variant="primary" :disabled="!canSave || saving" @click="save">
              {{ t('common.save') }}
            </Button>
          </div>
        </div>
      </div>
    </PopoverContent>
  </Popover>
</template>

<style scoped>
@reference "../../style.css";

/* The panel is placed by the caller's own anchor; this is the zero-size point it hangs from. */
.anchor {
  @apply block w-0 h-0;
}

/* The panel's own box. It is a box of this component's template and not the popover's root for the
   reason the popover is portaled: this file's scope id never lands on that root. It clips, because a
   step slides in from the side. */
.sheet-body {
  @apply flex flex-col gap-0.5 p-2 overflow-hidden;
  width: 320px;
}

.step {
  @apply flex flex-col gap-0.5;
}

/* A step arrives from the side it stands on: the name comes in from the right of the place it was
   picked from, and the place comes back from the left. The two are one movement, and the panel is
   never empty in between. */
.step-next {
  animation: step-next 180ms ease-out;
}

.step-back {
  animation: step-back 180ms ease-out;
}

.head {
  @apply text-[11px] font-semibold uppercase tracking-[0.07em] text-text-tertiary;
  padding: 6px 10px 8px;
}

/* The field the search and the new collection share: what is typed filters the list and, when it
   names nothing, is the name the first row would make. */
.search {
  @apply flex items-center gap-2 h-8 rounded-[8px] text-text-tertiary;
  margin: 0 2px 6px;
  padding: 0 10px;
  background: var(--bg-inset);
  border: 1px solid var(--border-strong);
}

.search:focus-within {
  border-color: var(--accent);
}

.search input {
  @apply flex-1 min-w-0 bg-transparent border-0 outline-none text-[13px] text-text;
  font: inherit;
  font-size: 13px;
}

/* The list scrolls rather than the panel: the field and the foot are what a person aims at, and a
   tree of forty collections must not carry them off the screen. */
.places {
  @apply flex flex-col gap-0.5 overflow-y-auto;
  max-height: 232px;
  margin: 0 -2px;
  padding: 0 2px;
}

.place {
  @apply flex items-center gap-2.5 w-full min-h-9 px-2.5 rounded-[9px] border-none bg-transparent text-text text-left cursor-pointer;
  font: inherit;
  --wails-draggable: no-drag;
}

.place:hover {
  background: var(--bg-hover);
}

.place.active {
  @apply bg-accent-soft;
}

.place-mark {
  @apply flex-none w-4 inline-flex items-center justify-center text-accent;
}

.place-text {
  @apply flex-1 min-w-0 flex flex-col gap-0.5;
}

.place-name {
  @apply text-[13px] font-medium truncate;
}

.place.active .place-name {
  @apply font-semibold;
}

.place-meta {
  @apply text-[11px] text-text-tertiary truncate;
}

.no-places {
  @apply py-3.5 px-2.5 text-[12px] text-text-tertiary;
}

.confirm-head {
  @apply flex items-center gap-1.5;
  padding: 2px 6px 10px;
}

.back {
  @apply flex-none inline-flex items-center justify-center w-6 h-6 rounded-md border-none bg-transparent text-text-secondary cursor-pointer;
  --wails-draggable: no-drag;
}

.back:hover {
  @apply bg-bg-hover text-text;
}

/* The name of the place, truncated from the left: what tells one folder of a collection from another
   is its end, and «Example API / Articles» cut at the right would leave the two words that say
   nothing. */
.target {
  @apply flex-1 min-w-0 text-[13px] font-semibold overflow-hidden text-ellipsis whitespace-nowrap;
  direction: rtl;
  text-align: left;
}

.name-input {
  @apply w-full h-9 text-[13px] font-medium text-text rounded-[8px] outline-none;
  box-sizing: border-box;
  margin: 0 0 8px;
  padding: 0 11px;
  background: var(--bg-inset);
  border: 1px solid var(--border-strong);
}

.name-input:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.name-hint {
  @apply text-[11.5px] text-text-tertiary;
  padding: 0 4px 4px;
  line-height: 1.5;
}

.foot {
  @apply flex items-center gap-2 mt-1.5;
  padding: 10px 4px 2px;
  border-top: 1px solid color-mix(in srgb, var(--border) 60%, transparent);
}

.foot-state {
  @apply flex-1 min-w-0 text-[12px] text-text-tertiary truncate;
}

/* The one button the panel ends with: the handoff's 30 tall at the radius that goes with it. Named
   with the primitive's class as well, because a scoped override and the primitive's own scoped rule
   stand on the same specificity. */
.btn.save-btn {
  height: 30px;
  padding: 0 13px;
  border-radius: 8px;
  font-weight: 600;
}
</style>
