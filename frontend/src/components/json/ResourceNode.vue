<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Icon from '../ui/Icon.vue'
import { resourceKey, resourceLabel, type Resource, type Relationship } from '../../lib/jsonapi'
import RelationshipLink from './RelationshipLink.vue'
import { copyToClipboard } from '../../lib/export'

const props = defineProps<{
  resource: Resource
  index: Map<string, Resource>
  highlighted?: boolean
  path: string
}>()

const emit = defineEmits<{
  (e: 'jump', key: string): void
  (e: 'fetch', url: string): void
  (e: 'inspect', path: string): void
}>()

const open = ref(false)

function toggleOpen() {
  open.value = !open.value
  emit('inspect', props.path)
}

function relPath(name: string): string {
  return props.path + '.relationships.' + name
}

// A highlight from the map must expand the node, not just show its header.
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

// Click-to-copy is skipped during an active text selection, so a click-drag
// meant to select part of the value is not stolen.
const copiedKeys = ref<Set<string>>(new Set())
const copiedVals = ref<Set<string>>(new Set())

function hasSelection(): boolean {
  return (window.getSelection()?.toString().length ?? 0) > 0
}

function flash(target: 'key' | 'val', k: string) {
  const set = target === 'key' ? copiedKeys : copiedVals
  const next = new Set(set.value)
  next.add(k)
  set.value = next
  setTimeout(() => {
    const after = new Set(set.value)
    after.delete(k)
    set.value = after
  }, 700)
}

async function copyKey(k: string) {
  if (hasSelection()) return
  if (await copyToClipboard(k)) flash('key', k)
}

async function copyVal(k: string, v: unknown) {
  if (hasSelection()) return
  const text = v === null ? 'null' : typeof v === 'object' ? JSON.stringify(v, null, 2) : String(v)
  if (await copyToClipboard(text)) flash('val', k)
}
</script>

<template>
  <div class="ja-resource" :id="rid" :class="{ highlight: highlighted }">
    <button class="ja-resource-head" @click="toggleOpen">
      <span class="ja-caret" :class="{ open }">
        <Icon name="chevron-right" :size="10" />
      </span>
      <span class="ja-type-badge">{{ resource.type }}</span>
      <span class="ja-id">{{ resource.id }}</span>
      <span class="ja-label">{{ label }}</span>
    </button>

    <div v-if="open" class="ja-body">
      <div v-if="attributes.length" class="ja-section-title" style="padding-left: 0">attributes</div>
      <div v-for="[k, v] in attributes" :key="k" class="ja-attr">
        <span class="ja-attr-key" title="Скопировать ключ" :class="{ copied: copiedKeys.has(k) }" @click.stop="copyKey(k)">{{ k }}</span>
        <span
          class="ja-attr-val"
          title="Скопировать значение"
          :class="[valueClass(v), { copied: copiedVals.has(k) }]"
          @click.stop="copyVal(k, v)"
          >{{ formatValue(v) }}</span
        >
      </div>

      <template v-if="relationships.length">
        <div class="ja-section-title" style="padding-left: 0">relationships</div>
        <div v-for="[name, rel] in relationships" :key="name" class="ja-rel">
          <span class="ja-rel-name">{{ name }}</span>
          <RelationshipLink :rel="rel" :index="index" @jump="(k) => emit('jump', k)" @fetch="(u) => emit('fetch', u)" @inspect="emit('inspect', relPath(name))" />
        </div>
      </template>
    </div>
  </div>
</template>
