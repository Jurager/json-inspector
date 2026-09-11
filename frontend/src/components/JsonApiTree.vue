<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import {
  buildIndex,
  dataResources,
  resourceMatchesQuery,
  type JsonApiDocument,
  type Resource,
} from '../lib/jsonapi'
import ResourceNode from './ResourceNode.vue'
import Icon from './Icon.vue'

const props = withDefaults(
  defineProps<{
    doc: JsonApiDocument
    highlightKey?: string | null
    // Search lives in the parent's toolbar (the same row as pagination) —
    // this component just filters by whatever query it's handed.
    query?: string
  }>(),
  { query: '' }
)

const emit = defineEmits<{
  (e: 'fetch', url: string): void
  (e: 'select', key: string): void
  (e: 'inspect', path: string): void
}>()

const index = computed(() => buildIndex(props.doc))
const data = computed(() => dataResources(props.doc))
const included = computed(() => props.doc.included ?? [])
const errors = computed(() => props.doc.errors ?? [])

// Russian plural for the section counts ("ресурс/ресурса/ресурсов").
function pluralRu(n: number, forms: [string, string, string]): string {
  const m10 = n % 10
  const m100 = n % 100
  if (m10 === 1 && m100 !== 11) return forms[0]
  if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return forms[1]
  return forms[2]
}

// Flat index in doc.included, so a grouped resource still gets a stable
// "included[j]" path for the inspector.
const includedFlatIndex = computed(() => {
  const map = new Map<string, number>()
  included.value.forEach((r, i) => map.set(r.type + '/' + r.id, i))
  return map
})

function includedPath(r: Resource): string {
  return 'included[' + (includedFlatIndex.value.get(r.type + '/' + r.id) ?? 0) + ']'
}

const highlightedKey = ref<string | null>(null)

// Included can be a 94-resource flat list — unreadable — so it's grouped by
// type. Groups collapse only when there are enough resources to matter; search
// and a selected type chip always force them open.
const typeFilter = ref<string | null>(null)
const expandedGroups = ref<Set<string>>(new Set())
const showAllTypes = ref(false)

function toggleTypeFilter(type: string) {
  typeFilter.value = typeFilter.value === type ? null : type
}

function isGroupOpen(type: string): boolean {
  if (props.query.trim() || typeFilter.value) return true
  if (included.value.length <= 20) return true
  return expandedGroups.value.has(type)
}

function toggleGroup(type: string) {
  const next = new Set(expandedGroups.value)
  if (next.has(type)) next.delete(type)
  else next.add(type)
  expandedGroups.value = next
}

function forceExpand(type: string) {
  if (!expandedGroups.value.has(type)) {
    const next = new Set(expandedGroups.value)
    next.add(type)
    expandedGroups.value = next
  }
}

function highlightAndScroll(key: string) {
  highlightedKey.value = key
  // A relationship can jump into a collapsed group — expand it so the scroll
  // target actually exists.
  forceExpand(key.split('/')[0])
  nextTick(() => {
    const el = document.getElementById('res-' + key)
    el?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  })
}

watch(
  () => props.highlightKey,
  (k) => {
    if (k) highlightAndScroll(k)
  }
)

// The tree remounts when switching back from the map tab; jump on mount so a
// highlight set while on the map still lands.
onMounted(() => {
  if (props.highlightKey) highlightAndScroll(props.highlightKey)
})

function jumpTo(key: string) {
  highlightAndScroll(key)
  emit('select', key)
}

function isHighlighted(key: string): boolean {
  return highlightedKey.value === key
}

const jsonapiVersion = computed(() => {
  const j = props.doc.jsonapi
  if (j && typeof j === 'object' && !Array.isArray(j)) {
    const v = (j as Record<string, unknown>).version
    if (typeof v === 'string' && v) return v
  }
  return ''
})

const metaText = computed(() => {
  const m = props.doc.meta
  return m == null ? '' : JSON.stringify(m, null, 2)
})

const isFiltering = computed(() => props.query.trim().length > 0)

const filteredData = computed(() => {
  const q = props.query.trim().toLowerCase()
  if (!q) return data.value
  return data.value.filter((r) => resourceMatchesQuery(r, q))
})

const filteredIncluded = computed(() => {
  const q = props.query.trim().toLowerCase()
  if (!q) return included.value
  return included.value.filter((r) => resourceMatchesQuery(r, q))
})

// Type summary chips in the included header — most populous types first.
const typeCounts = computed(() => {
  const map = new Map<string, number>()
  for (const r of filteredIncluded.value) {
    map.set(r.type, (map.get(r.type) ?? 0) + 1)
  }
  return Array.from(map.entries())
    .map(([type, count]) => ({ type, count }))
    .sort((a, b) => b.count - a.count)
})

const visibleTypeChips = computed(() => (showAllTypes.value ? typeCounts.value : typeCounts.value.slice(0, 4)))
const moreTypes = computed(() => Math.max(0, typeCounts.value.length - 4))

const includedGroups = computed(() => {
  const map = new Map<string, Resource[]>()
  for (const r of filteredIncluded.value) {
    if (typeFilter.value && r.type !== typeFilter.value) continue
    const list = map.get(r.type) ?? []
    list.push(r)
    map.set(r.type, list)
  }
  return Array.from(map.entries()).map(([type, resources]) => ({ type, resources }))
})

const noResults = computed(
  () => isFiltering.value && filteredData.value.length === 0 && filteredIncluded.value.length === 0
)
</script>

<template>
  <div class="ja-root">
    <div v-if="errors.length">
      <div class="ja-section-title">errors</div>
      <pre class="code px-4">{{ JSON.stringify(errors, null, 2) }}</pre>
    </div>

    <div v-if="jsonapiVersion" class="ja-link">
      <span class="ja-link-name">jsonapi</span>
      <span class="ja-meta-value mono">v{{ jsonapiVersion }}</span>
    </div>

    <div v-if="metaText">
      <div class="ja-section-title">meta</div>
      <pre class="code px-4">{{ metaText }}</pre>
    </div>

    <div v-if="noResults" class="ja-no-results">Ничего не найдено</div>

    <template v-if="filteredData.length">
      <div class="ja-section-title">
        data · {{ filteredData.length }} {{ pluralRu(filteredData.length, ['ресурс', 'ресурса', 'ресурсов']) }}
      </div>
      <ResourceNode
        v-for="(r, i) in filteredData"
        :key="r.type + '/' + r.id"
        :resource="r"
        :index="index"
        :path="'data[' + i + ']'"
        :highlighted="isHighlighted(r.type + '/' + r.id)"
        @jump="jumpTo"
        @fetch="(u) => emit('fetch', u)"
        @inspect="(p) => emit('inspect', p)"
      />
    </template>

    <template v-if="filteredIncluded.length">
      <div class="ja-section-title ja-included-head">
        <span>
          included · {{ filteredIncluded.length }} {{ pluralRu(filteredIncluded.length, ['ресурс', 'ресурса', 'ресурсов']) }}
          · {{ typeCounts.length }} {{ pluralRu(typeCounts.length, ['тип', 'типа', 'типов']) }}
        </span>
        <span v-if="typeCounts.length" class="type-chips">
          <button
            v-for="tc in visibleTypeChips"
            :key="tc.type"
            class="type-chip"
            :class="{ active: typeFilter === tc.type }"
            @click="toggleTypeFilter(tc.type)"
          >
            {{ tc.type }} {{ tc.count }}
          </button>
          <button v-if="moreTypes && !showAllTypes" class="type-chip more" @click="showAllTypes = true">
            ещё {{ moreTypes }}
          </button>
        </span>
      </div>

      <div v-for="g in includedGroups" :key="g.type" class="included-group">
        <button class="included-group-head" @click="toggleGroup(g.type)">
          <span class="ja-caret" :class="{ open: isGroupOpen(g.type) }">
            <Icon name="chevron-right" :size="10" />
          </span>
          <span class="ja-type-badge">{{ g.type }}</span>
          <span class="group-res-count">
            {{ g.resources.length }} {{ pluralRu(g.resources.length, ['ресурс', 'ресурса', 'ресурсов']) }}
          </span>
        </button>
        <div v-if="isGroupOpen(g.type)">
          <ResourceNode
            v-for="r in g.resources"
            :key="r.type + '/' + r.id"
            :resource="r"
            :index="index"
            :path="includedPath(r)"
            :highlighted="isHighlighted(r.type + '/' + r.id)"
            @jump="jumpTo"
            @fetch="(u) => emit('fetch', u)"
            @inspect="(p) => emit('inspect', p)"
          />
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
@reference "../style.css";

.ja-no-results {
  @apply p-4 text-center text-text-tertiary text-xs;
}

.ja-included-head {
  @apply flex items-center gap-2 flex-wrap;
}

.type-chips {
  @apply inline-flex flex-wrap gap-1;
}

.type-chip {
  @apply text-[10.5px] text-purple py-px px-[7px] rounded-sm border-0 cursor-pointer;
  font-family: var(--mono);
  background: color-mix(in srgb, var(--purple) 14%, transparent);
}

.type-chip.active {
  outline: 1px solid var(--purple);
}

.type-chip.more {
  color: var(--text-secondary);
  background: var(--bg-hover);
}

.included-group {
  @apply mb-1;
}

/* An included type group is a card of its own — the same white surface as a
   resource card — so the collapsed groups read as peers of the resources they
   contain, not as bare section labels. */
.included-group-head {
  @apply flex items-center gap-2 w-full py-2 px-3 rounded-lg text-left cursor-pointer select-none;
  margin: 5px 16px;
  border: 1px solid var(--border);
  background: var(--bg-panel);
  --wails-draggable: no-drag;
}

.included-group-head:hover {
  @apply bg-bg-hover;
}

.group-res-count {
  @apply text-xs text-text-secondary;
}
</style>
