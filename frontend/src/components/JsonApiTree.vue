<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import {
  buildIndex,
  dataResources,
  resourceMatchesQuery,
  type JsonApiDocument,
} from '../lib/jsonapi'
import ResourceNode from './ResourceNode.vue'

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
}>()

const index = computed(() => buildIndex(props.doc))
const data = computed(() => dataResources(props.doc))
const included = computed(() => props.doc.included ?? [])
const errors = computed(() => props.doc.errors ?? [])

const highlightedKey = ref<string | null>(null)

function highlightAndScroll(key: string) {
  highlightedKey.value = key
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
      <div class="ja-section-title">data</div>
      <ResourceNode
        v-for="r in filteredData"
        :key="r.type + '/' + r.id"
        :resource="r"
        :index="index"
        :highlighted="isHighlighted(r.type + '/' + r.id)"
        @jump="jumpTo"
        @fetch="(u) => emit('fetch', u)"
      />
    </template>

    <template v-if="filteredIncluded.length">
      <div class="ja-section-title">
        included {{ isFiltering ? `(${filteredIncluded.length} / ${included.length})` : `(${included.length})` }}
      </div>
      <ResourceNode
        v-for="r in filteredIncluded"
        :key="r.type + '/' + r.id"
        :resource="r"
        :index="index"
        :highlighted="isHighlighted(r.type + '/' + r.id)"
        @jump="jumpTo"
        @fetch="(u) => emit('fetch', u)"
      />
    </template>
  </div>
</template>

<style scoped>
@reference "../style.css";

.ja-no-results {
  @apply p-4 text-center text-text-tertiary text-xs;
}
</style>
