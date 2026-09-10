<script setup lang="ts">
import { computed, ref } from 'vue'

const props = withDefaults(
  defineProps<{
    value: unknown
    name?: string
    depth?: number
  }>(),
  { depth: 0 }
)

const collapsed = ref((props.depth ?? 0) >= 2)

const isObject = computed(() => props.value !== null && typeof props.value === 'object')
const isArray = computed(() => Array.isArray(props.value))

const entries = computed(() => {
  const v = props.value
  if (Array.isArray(v)) return v.map((item, i) => ({ key: String(i), value: item }))
  if (v !== null && typeof v === 'object') {
    return Object.entries(v as Record<string, unknown>).map(([k, val]) => ({ key: k, value: val }))
  }
  return []
})

const preview = computed(() => {
  const v = props.value
  if (Array.isArray(v)) return `${v.length}`
  if (v !== null && typeof v === 'object') return `${Object.keys(v).length}`
  return ''
})
</script>

<template>
  <div class="jt-node">
    <span v-if="!isObject">
      <span v-if="name !== undefined" class="jt-key">{{ name }}: </span>
      <span v-if="value === null" class="jt-null">null</span>
      <span v-else-if="typeof value === 'string'" class="jt-string">"{{ value }}"</span>
      <span v-else-if="typeof value === 'number'" class="jt-number">{{ value }}</span>
      <span v-else-if="typeof value === 'boolean'" class="jt-boolean">{{ value }}</span>
      <span v-else>{{ value }}</span>
    </span>

    <template v-else>
      <div>
        <span class="jt-toggle" @click="collapsed = !collapsed">{{ collapsed ? '▸' : '▾' }}</span>
        <span v-if="name !== undefined" class="jt-key">{{ name }}</span>
        <span class="jt-bracket">{{ isArray ? '[' : '{' }}</span>
        <span v-if="collapsed" class="jt-bracket">{{ isArray ? ']' : '}' }}</span>
        <span v-if="collapsed && preview" class="jt-key" style="opacity: 0.45"> {{ preview }}</span>
      </div>
      <div v-if="!collapsed" class="jt-children">
        <JsonTree
          v-for="(e, i) in entries"
          :key="i"
          :name="e.key"
          :value="e.value"
          :depth="(depth ?? 0) + 1"
        />
        <div class="jt-bracket">{{ isArray ? ']' : '}' }}</div>
      </div>
    </template>
  </div>
</template>
