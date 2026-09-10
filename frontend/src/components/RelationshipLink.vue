<script setup lang="ts">
import { computed } from 'vue'
import {
  relIdentifiers,
  resourceKey,
  href,
  resourceLabel,
  type Relationship,
  type Resource,
} from '../lib/jsonapi'

const props = defineProps<{
  rel: Relationship
  index: Map<string, Resource>
}>()

const emit = defineEmits<{
  (e: 'jump', key: string): void
  (e: 'fetch', url: string): void
}>()

const targets = computed(() => relIdentifiers(props.rel))

function labelFor(type: string, id: string): string {
  const r = props.index.get(resourceKey(type, id))
  return r ? resourceLabel(r) : `${type}/${id}`
}
</script>

<template>
  <span v-if="targets.length === 0" class="rel-chip-empty">пусто</span>
  <span v-else class="ja-rel-targets">
    <template v-for="(t, i) in targets" :key="i">
      <button
        v-if="index.has(resourceKey(t.type, t.id))"
        class="rel-chip in-doc"
        :title="resourceKey(t.type, t.id)"
        @click="emit('jump', resourceKey(t.type, t.id))"
      >
        {{ labelFor(t.type, t.id) }}
      </button>
      <button
        v-else-if="href(rel.links?.related)"
        class="rel-chip fetchable"
        :title="`fetch ${href(rel.links?.related)}`"
        @click="emit('fetch', href(rel.links?.related))"
      >
        {{ t.type }}/{{ t.id }} ↗
      </button>
      <span v-else class="rel-chip missing" :title="'не включён в документ'">
        {{ t.type }}/{{ t.id }}
      </span>
    </template>
  </span>
</template>
