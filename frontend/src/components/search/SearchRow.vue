<script setup lang="ts">
import { computed } from 'vue'
import { SearchKind } from '../../../bindings/json-inspector/internal/domain'
import type { PaletteRow } from '../../stores/search'
import { highlightMatch } from '../../lib/highlightMatch'
import { kindIcon, noteText } from '../../lib/searchKinds'
import { formatAgo } from '../../i18n'
import Icon from '../ui/Icon.vue'

const props = defineProps<{ row: PaletteRow; query: string; active: boolean }>()
defineEmits<{ (e: 'activate'): void }>()

// The breadcrumb is glued here rather than in Go: the separator is punctuation, and both sides of it
// are the user's own names.
const trail = computed(() => {
  const path = props.row.hit?.path ?? []
  return path.filter((step) => step !== '').join(' › ')
})

// A command has no method and no place: it is a word the window knows how to say and how to do.
const icon = computed(() => props.row.command?.icon ?? kindIcon(props.row.hit?.kind ?? SearchKind.SearchAction))

const label = computed(() => props.row.command?.label ?? props.row.hit?.title ?? '')
const badge = computed(() => props.row.hit?.badge ?? '')

// What the row says on the far right: the note a shape asked for, or — for a row that happened, which
// only the history has — how long ago. The two never meet, and one line holds the one it has.
const note = computed(() => {
  const shaped = noteText(props.row.hit?.note)
  if (shaped) return shaped
  const at = props.row.hit?.at
  return at ? formatAgo(at) : ''
})
</script>

<template>
  <div class="search-row" :class="{ 'search-row-active': active }" @click="$emit('activate')">
    <span v-if="badge" class="search-badge">{{ badge }}</span>
    <Icon v-else :name="icon" :size="15" class="search-glyph" />
    <!-- The title and the trail are the user's own text, so what is drawn is escaped by the
         highlighter and never taken for markup. -->
    <span class="search-title" :class="{ 'search-title-mono': !!badge }" v-html="highlightMatch(label, query)" />
    <span v-if="trail" class="search-trail" v-html="highlightMatch(trail, query)" />
    <span v-if="note" class="search-note">{{ note }}</span>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.search-row {
  @apply flex items-center gap-2.5 mx-2 px-2 py-[7px] rounded-md cursor-default;
  transition: background 0.15s ease;
}

/* The row the arrows are on keeps its own colour under the cursor: it is already the answer, and a
   hover that outshone it would move the eye off the row Enter is pointed at. */
.search-row:hover:not(.search-row-active) {
  background: var(--bg-hover);
}

.search-row-active {
  background: var(--accent-soft);
}

.search-badge {
  @apply inline-flex items-center justify-center flex-none w-11 h-5 rounded-[5px] text-[10px] font-bold;
  font-family: var(--mono);
  color: var(--accent);
  background: var(--accent-soft);
}

.search-glyph {
  @apply flex-none text-accent;
}

.search-title {
  @apply flex-none text-[13px] text-text truncate;
}

/* An address is machine text, and the design draws it in the mono face the rest of the app writes
   addresses in. */
.search-title-mono {
  @apply flex-1 min-w-0 text-[13px];
  font-family: var(--mono);
}

.search-trail {
  @apply flex-none text-[11px] text-text-tertiary truncate max-w-[190px];
}

.search-note {
  @apply flex-none ml-auto text-[11px] text-text-tertiary whitespace-nowrap;
}
</style>
