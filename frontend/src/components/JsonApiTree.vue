<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import {
  buildIndex,
  dataResources,
  type JsonApiDocument,
} from '../lib/jsonapi'
import ResourceNode from './ResourceNode.vue'

const props = defineProps<{
  doc: JsonApiDocument
  highlightKey?: string | null
}>()

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
</script>

<template>
  <div class="ja-root">
    <div v-if="errors.length">
      <div class="ja-section-title">errors</div>
      <pre class="code" style="padding: 0 16px">{{ JSON.stringify(errors, null, 2) }}</pre>
    </div>

    <div v-if="jsonapiVersion" class="ja-link">
      <span class="ja-link-name">jsonapi</span>
      <span class="ja-meta-value mono">v{{ jsonapiVersion }}</span>
    </div>

    <div v-if="metaText">
      <div class="ja-section-title">meta</div>
      <pre class="code" style="padding: 0 16px">{{ metaText }}</pre>
    </div>

    <template v-if="data.length">
      <div class="ja-section-title">data</div>
      <ResourceNode
        v-for="r in data"
        :key="r.type + '/' + r.id"
        :resource="r"
        :index="index"
        :highlighted="isHighlighted(r.type + '/' + r.id)"
        @jump="jumpTo"
        @fetch="(u) => emit('fetch', u)"
      />
    </template>

    <template v-if="included.length">
      <div class="ja-section-title">included ({{ included.length }})</div>
      <ResourceNode
        v-for="r in included"
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
