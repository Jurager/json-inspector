<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from './Icon.vue'
import { copyToClipboard } from '../lib/export'

const props = withDefaults(
  defineProps<{
    value: unknown
    name?: string
    path?: string
    depth?: number
  }>(),
  { depth: 0, path: '' }
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

const IDENT_RE = /^[A-Za-z_$][A-Za-z0-9_$]*$/

function childPath(key: string, arrayChild: boolean): string {
  if (arrayChild) return `${props.path}[${key}]`
  if (IDENT_RE.test(key)) return props.path ? `${props.path}.${key}` : key
  return `${props.path}["${key.replace(/"/g, '\\"')}"]`
}

// Click-to-copy on the key (copies its JSON path) or the value (copies the
// value itself). Skipped while the user has an active text selection, so it
// doesn't steal a click-drag meant to select part of the text.
const copiedKey = ref(false)
const copiedValue = ref(false)

function hasSelection(): boolean {
  return (window.getSelection()?.toString().length ?? 0) > 0
}

async function copyKeyPath() {
  if (!props.path || hasSelection()) return
  if (await copyToClipboard(props.path)) {
    copiedKey.value = true
    setTimeout(() => (copiedKey.value = false), 700)
  }
}

function valueText(): string {
  const v = props.value
  if (v === null) return 'null'
  if (typeof v === 'object') return JSON.stringify(v, null, 2)
  return String(v)
}

async function copyValue() {
  if (hasSelection()) return
  if (await copyToClipboard(valueText())) {
    copiedValue.value = true
    setTimeout(() => (copiedValue.value = false), 700)
  }
}
</script>

<template>
  <div class="jt-node">
    <span v-if="!isObject">
      <span
        v-if="name !== undefined"
        class="jt-key"
        title="Скопировать путь"
        :class="{ copied: copiedKey }"
        @click.stop="copyKeyPath"
        >{{ name }}: </span>
      <span class="jt-value" title="Скопировать значение" :class="{ copied: copiedValue }" @click.stop="copyValue">
        <span v-if="value === null" class="jt-null">null</span>
        <span v-else-if="typeof value === 'string'" class="jt-string">"{{ value }}"</span>
        <span v-else-if="typeof value === 'number'" class="jt-number">{{ value }}</span>
        <span v-else-if="typeof value === 'boolean'" class="jt-boolean">{{ value }}</span>
        <span v-else>{{ value }}</span>
      </span>
    </span>

    <template v-else>
      <div>
        <span class="jt-toggle" @click="collapsed = !collapsed"><Icon :name="collapsed ? 'chevron-right' : 'chevron-down'" :size="10" /></span>
        <span
          v-if="name !== undefined"
          class="jt-key"
          title="Скопировать путь"
          :class="{ copied: copiedKey }"
          @click.stop="copyKeyPath"
          >{{ name }}</span>
        <span class="jt-value" title="Скопировать значение" :class="{ copied: copiedValue }" @click.stop="copyValue">
          <span class="jt-bracket">{{ isArray ? '[' : '{' }}</span>
          <span v-if="collapsed" class="jt-bracket">{{ isArray ? ']' : '}' }}</span>
          <span v-if="collapsed && preview" class="jt-preview"> {{ preview }}</span>
        </span>
      </div>
      <div v-if="!collapsed" class="jt-children">
        <JsonTree
          v-for="(e, i) in entries"
          :key="i"
          :name="e.key"
          :value="e.value"
          :path="childPath(e.key, isArray)"
          :depth="(depth ?? 0) + 1"
        />
        <div class="jt-bracket">{{ isArray ? ']' : '}' }}</div>
      </div>
    </template>
  </div>
</template>
