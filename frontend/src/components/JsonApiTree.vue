<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import {
  buildIndex,
  dataResources,
  href,
  type JsonApiDocument,
} from '../lib/jsonapi'
import { decodeUrl } from '../lib/json'
import ResourceNode from './ResourceNode.vue'

const props = defineProps<{
  doc: JsonApiDocument
  highlightKey?: string | null
}>()

const emit = defineEmits<{
  (e: 'fetch', url: string): void
}>()

const index = computed(() => buildIndex(props.doc))
const data = computed(() => dataResources(props.doc))
const included = computed(() => props.doc.included ?? [])
const errors = computed(() => props.doc.errors ?? [])

const highlightedKey = ref<string | null>(null)

watch(
  () => props.highlightKey,
  (k) => {
    if (k) jumpTo(k)
  }
)

function jumpTo(key: string) {
  highlightedKey.value = key
  nextTick(() => {
    const el = document.getElementById('res-' + key)
    el?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  })
}

function isHighlighted(key: string): boolean {
  return highlightedKey.value === key
}

const docLinks = computed<[string, string][]>(() =>
  Object.entries(props.doc.links ?? {})
    .map(([k, v]) => [k, href(v)] as [string, string])
    .filter(([, url]) => url !== '')
)
</script>

<template>
  <div class="ja-root">
    <div v-if="errors.length">
      <div class="ja-section-title">errors</div>
      <pre class="code" style="padding: 0 16px">{{ JSON.stringify(errors, null, 2) }}</pre>
    </div>

    <div v-if="docLinks.length">
      <div class="ja-section-title">links</div>
      <div v-for="[name, url] in docLinks" :key="name" class="ja-link">
        <span class="ja-link-name">{{ name }}</span>
        <button class="ja-link-url" :title="decodeUrl(url)" @click="emit('fetch', url)">
          {{ decodeUrl(url) }}
        </button>
      </div>
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
