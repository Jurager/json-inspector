<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import Icon from '../ui/Icon.vue'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Popover, PopoverAnchor, PopoverContent, PopoverClose } from '../ui/popover'
import { useCollectionsStore } from '../../stores/collections'
import { describeFailure, useMessages } from '../../i18n'
import { useToast } from '../../composables/useToast'
import type { Collection } from '../../../bindings/json-inspector/internal/domain'

const { t } = useMessages()

// Where a request composed in the command line goes when it is saved: a collection, in the order the
// tree draws them. It is not the tree itself — the sheet offers what a request can live in, and a
// request lives in a collection.
const props = defineProps<{ open: boolean; url: string; defaultName: string }>()
const emit = defineEmits<{ (e: 'update:open', value: boolean): void }>()

const store = useCollectionsStore()
const toast = useToast()

interface Place {
  id: string
  name: string
  depth: number
}

const places = computed<Place[]>(() => {
  const out: Place[] = []
  const walk = (collections: Collection[], depth: number) => {
    for (const collection of collections) {
      out.push({ id: collection.id, name: collection.name, depth })
      walk(collection.children ?? [], depth + 1)
    }
  }
  walk(store.tree, 0)
  return out
})

const selected = ref<Place | null>(null)
const name = ref('')
const nameInput = ref<HTMLInputElement | null>(null)
const saving = ref(false)

// The place chosen by default: the collection the user is looking at, so the sheet opens ready to
// accept rather than asking a question whose answer is usually the same.
watch(
  () => props.open,
  (open) => {
    if (!open) return
    name.value = props.defaultName
    const current = places.value.find((p) => p.id === store.collectionId) ?? places.value[0] ?? null
    selected.value = current
    nextTick(() => {
      nameInput.value?.focus()
      nameInput.value?.select()
    })
  }
)

const canSave = computed(() => !!selected.value && name.value.trim().length > 0 && props.url.trim().length > 0)

// The button this sheet hangs from stands outside its own content, so pressing it reaches reka-ui as
// an outside interaction and closes the sheet — and the button's `click`, running against that
// closed state, opens it again. The button could then never close what it opened, and a press held
// down flickered. Suppressing it for that button hands the whole decision to the button itself,
// which is where the flag lives.
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

const empty = computed(() => places.value.length === 0)

async function save() {
  const place = selected.value
  if (!place || !canSave.value || saving.value) return
  saving.value = true
  try {
    await store.saveDraft(place.id, name.value.trim())
    toast.show(t('collections.savedTo', { name: place.name }))
    emit('update:open', false)
  } catch (error) {
    toast.show(t('collections.saveFailed', { error: describeFailure(error) }), 'error')
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

    <PopoverContent
      class="save-sheet"
      align="end"
      @open-auto-focus.prevent
      @interact-outside="onInteractOutside"
    >
      <div class="sheet-body">
        <div class="head">{{ t('collections.saveRequest') }}</div>

        <Input
          ref="nameInput"
          v-model="name"
          size="sm"
          :placeholder="t('collections.requestName')"
          spellcheck="false"
          @keydown.enter.prevent="save"
          @keydown.escape.prevent="emit('update:open', false)"
        />

        <div class="list">
          <button
            v-for="place in places"
            :key="place.id"
            class="place"
            :class="{ active: place.id === selected?.id }"
            :style="{ paddingLeft: 10 + Math.min(place.depth, 2) * 18 + 'px' }"
            @click="selected = place"
          >
            <Icon name="folder" :size="place.depth === 0 ? 12 : 11" class="place-icon" />
            <span class="place-name">{{ place.name }}</span>
          </button>
          <div v-if="empty" class="no-places">{{ t('collections.createCollectionFirst') }}</div>
        </div>

        <div class="actions">
          <PopoverClose as-child>
            <Button @click="emit('update:open', false)">{{ t('common.cancel') }}</Button>
          </PopoverClose>
          <Button variant="primary" :disabled="!canSave || saving" @click="save">{{ t('common.save') }}</Button>
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
   reason the popover is portaled: this file's scope id never lands on that root. */
.sheet-body {
  @apply flex flex-col gap-2 p-2.5;
  width: 320px;
}

.head {
  @apply text-[12px] font-semibold;
}

.list {
  @apply flex flex-col gap-px max-h-[220px] overflow-y-auto p-[5px] rounded-[7px] border border-border;
}

.place {
  @apply flex items-center gap-1.5 h-[28px] pr-2.5 rounded-[7px] border-none bg-transparent text-text text-[13px] text-left cursor-pointer w-full;
  font: inherit;
}

.place:hover {
  @apply bg-bg-hover;
}

.place.active {
  @apply bg-accent-soft text-accent;
}

.place-icon {
  @apply flex-none text-text-tertiary;
}

.place.active .place-icon {
  @apply text-accent;
}

.place-name {
  @apply flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap;
}

.no-places {
  @apply py-3 text-center text-[12px] text-text-tertiary;
}

.actions {
  @apply flex justify-end gap-2 mt-1;
}
</style>
