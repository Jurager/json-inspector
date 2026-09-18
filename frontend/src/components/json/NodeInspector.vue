<script setup lang="ts">
import { computed } from 'vue'
import Icon from '../ui/Icon.vue'
import { IconButton } from '../ui/button'
import { useRequestsStore } from '../../stores/requests'
import { useCollectionsStore } from '../../stores/collections'
import { useMessages } from '../../i18n'
import {
  buildResourceIndex,
  dataResources,
  linkHref,
  relIdentifiers,
  type JsonApiDocument,
  type Resource,
} from '../../lib/jsonapi'
import type { InspectorHost } from '../../lib/requestSource'
import { copyToClipboard } from '../../lib/clipboard'
import { useResizableWidth } from '../../composables/useResizableWidth'

// Which node is open in the panel and how wide it is belongs to whoever drew it: a card is a
// collection's, and the pane beside the command line is the window's. It is told which one it is
// beside, the way the pane that holds it is — reading one store whatever the other says would leave
// a card's panel answering the command line's clicks.
const props = withDefaults(
  defineProps<{ doc: JsonApiDocument | null; source?: 'request' | 'browser' | 'collection' }>(),
  { source: 'request' }
)
const emit = defineEmits<{ (e: 'close'): void; (e: 'fetch', url: string): void }>()

const store: InspectorHost = props.source === 'collection' ? useCollectionsStore() : useRequestsStore()

const { t } = useMessages()

const width = computed({
  get: () => store.inspector.width,
  set: (v: number) => store.setInspector({ width: v }),
})

const { startDrag: startResize } = useResizableWidth(width, { min: 220, max: 520, side: 'right' })

const path = computed(() => store.inspector.path ?? '')

// The document node the inspector path points at, as far as it can be followed.
interface InspectedNode {
  targetType: string
  targetId: string
  relatedUrl: string
  inDoc: boolean
}

function resolveInspectedNode(): InspectedNode | null {
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
  const index = buildResourceIndex(props.doc)
  return {
    targetType: ri.type,
    targetId: ri.id,
    relatedUrl: linkHref(rel.links?.related),
    inDoc: index.has(`${ri.type}/${ri.id}`),
  }
}

const inspectedNode = computed(() => resolveInspectedNode())

const relationText = computed(() => {
  const r = inspectedNode.value
  if (!r) return '—'
  const named = { type: r.targetType, id: r.targetId }
  return r.inDoc ? t('json.inDocument', named) : t('json.needsRelated', named)
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
  const r = inspectedNode.value
  if (r && r.relatedUrl) emit('fetch', r.relatedUrl)
}

</script>

<template>
  <div class="inspector" :style="{ width: width + 'px' }">
    <div class="inspector-resize" @mousedown.prevent="startResize"></div>

    <div class="inspector-head">
      <span class="inspector-title">{{ t('json.inspector') }}</span>
      <IconButton :hint="t('common.close')" @click="emit('close')">
        <Icon name="xmark" :size="14" :stroke-width="2.2" />
      </IconButton>
    </div>

    <div class="inspector-body">
      <div class="block">
        <div class="block-title">{{ t('json.path') }}</div>
        <div class="block-path mono">{{ path || '—' }}</div>
      </div>

      <div class="block">
        <div class="block-title">{{ t('json.relationship') }}</div>
        <div class="block-text">{{ relationText }}</div>
      </div>

      <div class="block">
        <div class="block-title">{{ t('json.schema') }}</div>
        <div class="schema-row">
          <span class="dot" :class="jsonapiVersion ? 'dot-green' : 'dot-orange'"></span>
          <span>{{ jsonapiVersion ? t('json.matches', { version: jsonapiVersion }) : t('json.notJsonApi') }}</span>
        </div>
      </div>

      <div class="block actions">
        <div class="block-title">{{ t('json.actions') }}</div>
        <button class="inspector-action" :disabled="!inspectedNode?.relatedUrl" @click="openRelated">{{ t('json.openRelated') }}</button>
        <button class="inspector-action" @click="copyPath">{{ t('json.copyPath') }}</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

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
  @apply flex-none flex items-center justify-between h-[46px] px-2.5 pl-4 border-b border-border;
}

.inspector-title {
  @apply text-[13px] font-semibold;
}

/* The handoff's own close for this panel: 28 square rather than the 24 of the small button. Reached
   through :deep because the button is drawn inside the tooltip that hints it, where a rule of this
   component's own scope never lands. */
.inspector-head :deep(button.icon-btn) {
  width: 28px;
  height: 28px;
  border-radius: 6px;
}

.inspector-body {
  @apply flex-1 min-h-0 overflow-auto p-4 flex flex-col gap-[18px];
}

.block {
  @apply flex flex-col gap-1.5;
}

/* The buttons of a block stand a little further apart than the block's own title does. */
.block.actions {
  @apply gap-2;
}

.block-title {
  @apply text-[11px] font-semibold uppercase tracking-[0.07em] text-text-tertiary;
}

.block-path {
  @apply text-[13px] text-text break-all;
  line-height: 1.5;
  font-family: var(--mono);
}

.block-text {
  @apply text-[13px] text-text-secondary;
  line-height: 1.5;
}

.schema-row {
  @apply flex items-center gap-[9px] text-[13px] text-text;
}

.dot {
  @apply w-2 h-2 rounded-full flex-none;
}

.dot-green {
  background: var(--green);
}

.dot-orange {
  background: var(--orange);
}

.inspector-action {
  @apply block w-full text-left h-9 px-3 rounded-lg text-[13px] cursor-pointer;
  border: 1px solid var(--border-strong);
  background: var(--bg-inset);
  color: var(--text);
  --wails-draggable: no-drag;
}

.inspector-action:hover {
  @apply bg-bg-hover;
}

.inspector-action:disabled {
  @apply opacity-50 cursor-default;
}
</style>
