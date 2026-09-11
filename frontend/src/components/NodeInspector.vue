<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue'
import Icon from './Icon.vue'
import { useRequestsStore } from '../stores/requests'
import {
  buildIndex,
  dataResources,
  href,
  relIdentifiers,
  type JsonApiDocument,
  type Resource,
} from '../lib/jsonapi'
import { copyToClipboard } from '../lib/export'
import { makeSideResizer } from '../lib/resize'

const props = defineProps<{ doc: JsonApiDocument | null }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'fetch', url: string): void }>()

const store = useRequestsStore()

const width = computed({
  get: () => store.inspector.width,
  set: (v: number) => store.setInspector({ width: v }),
})
const resize = makeSideResizer(width, 220, 520)

const path = computed(() => store.inspector.path ?? '')

interface Resolved {
  targetType: string
  targetId: string
  relatedUrl: string
  inDoc: boolean
}

// Resolves the stored path (e.g. "data[0].relationships.author") back to a
// concrete relationship target so the "Связь" and "Действия" blocks can say
// something useful about it.
function resolve(): Resolved | null {
  const p = path.value
  if (!p || !props.doc) return null
  const m = p.match(/^(data|included)\[(\d+)\](?:\.relationships\.(.+))?$/)
  if (!m) return null
  const arr: Resource[] = m[1] === 'data' ? dataResources(props.doc) : (props.doc.included ?? [])
  const res = arr[Number(m[2])]
  if (!res) return null
  if (!m[3]) {
    return { targetType: res.type, targetId: res.id, relatedUrl: '', inDoc: true }
  }
  const rel = res.relationships?.[m[3]]
  if (!rel) return null
  const ri = relIdentifiers(rel)[0]
  if (!ri) return null
  const index = buildIndex(props.doc)
  return {
    targetType: ri.type,
    targetId: ri.id,
    relatedUrl: href(rel.links?.related),
    inDoc: index.has(`${ri.type}/${ri.id}`),
  }
}

const resolved = computed(() => resolve())

const relationText = computed(() => {
  const r = resolved.value
  if (!r) return '—'
  return r.inDoc
    ? `${r.targetType} · ${r.targetId} — есть в документе, дополнительный запрос не нужен.`
    : `${r.targetType} · ${r.targetId} — нужен запрос links.related.`
})

const jsonapiVersion = computed(() => {
  const j = props.doc?.jsonapi
  if (j && typeof j === 'object' && !Array.isArray(j)) {
    const v = (j as Record<string, unknown>).version
    if (typeof v === 'string' && v) return v
  }
  return ''
})

async function copyPath() {
  if (path.value) await copyToClipboard(path.value)
}

function openRelated() {
  const r = resolved.value
  if (r && r.relatedUrl) emit('fetch', r.relatedUrl)
}

onMounted(() => {
  window.addEventListener('mousemove', resize.move)
  window.addEventListener('mouseup', resize.stop)
})

onBeforeUnmount(() => {
  window.removeEventListener('mousemove', resize.move)
  window.removeEventListener('mouseup', resize.stop)
})
</script>

<template>
  <div class="inspector" :style="{ width: width + 'px' }">
    <div class="inspector-resize" @mousedown.prevent="resize.start"></div>

    <div class="inspector-head">
      <span class="inspector-title">Инспектор узла</span>
      <button class="btn icon-btn" title="Закрыть" @click="emit('close')"><Icon name="xmark" :size="14" /></button>
    </div>

    <div class="inspector-body">
      <div class="block">
        <div class="block-title">Путь</div>
        <div class="block-path mono">{{ path || '—' }}</div>
      </div>

      <div class="block">
        <div class="block-title">Связь</div>
        <div class="block-text">{{ relationText }}</div>
      </div>

      <div class="block">
        <div class="block-title">Схема</div>
        <div class="schema-row">
          <span class="dot" :class="jsonapiVersion ? 'dot-green' : 'dot-orange'"></span>
          <span>{{ jsonapiVersion ? `Соответствует JSON:API ${jsonapiVersion}` : 'Не JSON:API документ' }}</span>
        </div>
      </div>

      <div class="block">
        <div class="block-title">Действия</div>
        <button class="inspector-action" :disabled="!resolved?.relatedUrl" @click="openRelated">Открыть links.related</button>
        <button class="inspector-action" @click="copyPath">Скопировать путь</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../style.css";

.inspector {
  @apply relative flex-none flex flex-col h-full min-h-0;
  border-left: 1px solid var(--border);
  background: var(--bg-panel);
}

.inspector-resize {
  @apply absolute left-0 top-0 bottom-0 w-[5px] -ml-[5px] cursor-col-resize z-1 bg-transparent;
  transition: background 0.15s ease;
}

.inspector-resize:hover {
  background: var(--border-strong);
}

.inspector-head {
  @apply flex-none flex items-center justify-between h-[38px] px-2 pl-3.5 border-b border-border;
}

.inspector-title {
  @apply text-xs font-semibold text-text-secondary;
}

.inspector-body {
  @apply flex-1 min-h-0 overflow-auto p-3.5 flex flex-col gap-3.5;
}

.block {
  @apply flex flex-col gap-1;
}

.block-title {
  @apply text-[10px] uppercase tracking-wider text-text-tertiary;
  font-family: var(--mono);
}

.block-path {
  @apply text-[11.5px] text-text leading-relaxed break-all;
}

.block-text {
  @apply text-xs text-text leading-relaxed;
}

.schema-row {
  @apply flex items-center gap-1.5 text-xs text-text;
}

.dot {
  @apply w-[7px] h-[7px] rounded-full flex-none;
}

.dot-green {
  background: var(--green);
}

.dot-orange {
  background: var(--orange);
}

.inspector-action {
  @apply block w-full text-left rounded-md py-1.5 px-2.5 text-xs cursor-pointer;
  border: 1px solid var(--border-strong);
  background: var(--bg-panel);
  color: var(--text);
  --wails-draggable: no-drag;
}

.inspector-action + .inspector-action {
  @apply mt-1.5;
}

.inspector-action:hover {
  @apply bg-bg-hover;
}

.inspector-action:disabled {
  @apply opacity-50 cursor-default;
}
</style>
