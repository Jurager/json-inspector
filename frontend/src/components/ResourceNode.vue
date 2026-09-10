<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Icon from './Icon.vue'
import { resourceKey, resourceLabel, type Resource, type Relationship } from '../lib/jsonapi'
import RelationshipLink from './RelationshipLink.vue'

const props = defineProps<{
  resource: Resource
  index: Map<string, Resource>
  highlighted?: boolean
}>()

const emit = defineEmits<{
  (e: 'jump', key: string): void
  (e: 'fetch', url: string): void
}>()

const open = ref(false)

// When the parent highlights this node (e.g. jump from the map), expand it so
// the resource is actually visible, not just its header.
watch(
  () => props.highlighted,
  (h) => {
    if (h) open.value = true
  }
)

const label = computed(() => resourceLabel(props.resource))

const attributes = computed<[string, unknown][]>(() =>
  Object.entries(props.resource.attributes ?? {})
)

const relationships = computed<[string, Relationship][]>(() =>
  Object.entries(props.resource.relationships ?? {})
)

function valueClass(v: unknown): string {
  if (v === null) return 'null'
  if (typeof v === 'string') return 'str'
  if (typeof v === 'number') return 'num'
  if (typeof v === 'boolean') return 'bool'
  return ''
}

function formatValue(v: unknown): string {
  if (v === null) return 'null'
  if (typeof v === 'string') return JSON.stringify(v)
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}

const rid = computed(() => 'res-' + resourceKey(props.resource.type, props.resource.id))
</script>

<template>
  <div class="ja-resource" :id="rid" :class="{ highlight: highlighted }">
    <div class="ja-resource-head" @click="open = !open">
      <span class="ja-caret" :class="{ open }">
        <Icon name="chevron-right" :size="10" />
      </span>
      <span class="ja-type-badge">{{ resource.type }}</span>
      <span class="ja-id">{{ resource.id }}</span>
      <span class="ja-label">{{ label }}</span>
    </div>

    <div v-if="open" class="ja-body">
      <div v-if="attributes.length" class="ja-section-title" style="padding-left: 0">attributes</div>
      <div v-for="[k, v] in attributes" :key="k" class="ja-attr">
        <span class="ja-attr-key">{{ k }}</span>
        <span class="ja-attr-val" :class="valueClass(v)">{{ formatValue(v) }}</span>
      </div>

      <template v-if="relationships.length">
        <div class="ja-section-title" style="padding-left: 0">relationships</div>
        <div v-for="[name, rel] in relationships" :key="name" class="ja-rel">
          <span class="ja-rel-name">{{ name }}</span>
          <RelationshipLink :rel="rel" :index="index" @jump="(k) => emit('jump', k)" @fetch="(u) => emit('fetch', u)" />
        </div>
      </template>
    </div>
  </div>
</template>
