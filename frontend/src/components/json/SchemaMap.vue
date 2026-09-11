<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { DialogTitle } from 'reka-ui'
import Icon from '../ui/Icon.vue'
import Dialog from '../ui/dialog/Dialog.vue'
import { Button, IconButton } from '../ui/button'
import { Input } from '../ui/input'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from '../ui/dropdown-menu'
import type { JsonApiDocument, Resource } from '../../lib/jsonapi'
import { dataResources, isJsonApi, resourceKey, resourceLabel } from '../../lib/jsonapi'
import { buildSchema, diffSchemas, humanize, type TypeDiff } from '../../lib/schema'
import { copyToClipboard } from '../../lib/export'
import { tryParseJson } from '../../lib/json'
import { useRequestsStore } from '../../stores/requests'
import { shortcut } from '../../lib/platform'

const props = defineProps<{ doc: JsonApiDocument | null; highlightKey?: string | null }>()
const emit = defineEmits<{ (e: 'fetch', url: string): void; (e: 'select', key: string): void }>()

const store = useRequestsStore()

const all = computed<Resource[]>(() => {
  if (!props.doc) return []
  return [...dataResources(props.doc), ...(props.doc.included ?? [])]
})

const types = computed(() => (props.doc ? buildSchema(props.doc) : []))

const instancesByType = computed<Map<string, Resource[]>>(() => {
  const map = new Map<string, Resource[]>()
  for (const r of all.value) {
    const list = map.get(r.type) ?? []
    list.push(r)
    map.set(r.type, list)
  }
  return map
})

function instancesOf(type: string): Resource[] {
  return instancesByType.value.get(type) ?? []
}

function instanceLabel(res: Resource): string {
  const l = resourceLabel(res)
  return l === `${res.type}/${res.id}` ? res.id : l
}

const expandedTypes = ref<Set<string>>(new Set())

function toggleType(type: string) {
  const next = new Set(expandedTypes.value)
  if (next.has(type)) next.delete(type)
  else next.add(type)
  expandedTypes.value = next
}

const query = ref('')

const searchVisible = ref(false)
const searchInputRef = ref<InstanceType<typeof Input> | null>(null)
const searchShortcut = computed(() => shortcut('F'))

function openSearch() {
  searchVisible.value = true
  nextTick(() => searchInputRef.value?.focus())
}

function closeSearch() {
  searchVisible.value = false
  query.value = ''
}

function onWindowKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.code === 'KeyF') {
    e.preventDefault()
    openSearch()
  } else if (e.key === 'Escape' && searchVisible.value) {
    closeSearch()
  }
}

onMounted(() => window.addEventListener('keydown', onWindowKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onWindowKeydown))

const filteredTypes = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return types.value
  return types.value.filter((t) => {
    if (t.label.toLowerCase().includes(q) || t.type.toLowerCase().includes(q)) return true
    if (t.attributes.some((a) => a.toLowerCase().includes(q))) return true
    if (t.rels.some((r) => r.name.toLowerCase().includes(q) || r.targetType.toLowerCase().includes(q))) return true
    return false
  })
})

const highlightType = ref<string | null>(null)

function goToType(type: string) {
  highlightType.value = type
  document.querySelector(`[data-type="${type}"]`)?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
  setTimeout(() => {
    if (highlightType.value === type) highlightType.value = null
  }, 1500)
}

// --- Reverse sync: highlight the instance selected in the tree ---
const highlightInstance = ref<string | null>(null)

function applyHighlight(k: string | null | undefined) {
  if (!k) return
  const type = k.split('/')[0]
  const next = new Set(expandedTypes.value)
  if (!next.has(type)) {
    next.add(type)
    expandedTypes.value = next
  }
  highlightInstance.value = k
  nextTick(() => {
    document.getElementById('inst-' + k)?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
  })
}

watch(() => props.highlightKey, applyHighlight)
onMounted(() => applyHighlight(props.highlightKey))

// --- Export schema ---
const copied = ref(false)

const EXPORT_FORMATS = [
  { id: 'mermaid', label: 'Mermaid (erDiagram)' },
  { id: 'dot', label: 'Graphviz DOT' },
  { id: 'json', label: 'JSON' },
] as const

type ExportId = (typeof EXPORT_FORMATS)[number]['id']

function entityName(type: string): string {
  let s = type.toUpperCase().replace(/[^A-Z0-9]+/g, '_')
  if (/^[0-9]/.test(s)) s = '_' + s
  return s
}

function attrName(a: string): string {
  return a.replace(/[^A-Za-z0-9_]/g, '_') || 'field'
}

function dotEscape(s: string): string {
  return s.replace(/\\/g, '\\\\').replace(/"/g, '\\"')
}

function exportMermaid(): string {
  const L: string[] = ['erDiagram']
  for (const t of types.value) {
    L.push(`  ${entityName(t.type)} {`)
    for (const a of t.attributes) L.push(`    string ${attrName(a)}`)
    L.push('  }')
  }
  for (const t of types.value) {
    for (const r of t.rels) {
      if (!r.inDoc) continue
      const card = r.many ? '||--o{' : '||--o|'
      L.push(`  ${entityName(t.type)} ${card} ${entityName(r.targetType)} : ${r.name}`)
    }
  }
  return L.join('\n')
}

function exportDot(): string {
  const L: string[] = ['digraph {', '  rankdir=LR']
  for (const t of types.value) {
    const label = [t.label, ...t.attributes].join('\\n')
    L.push(`  "${dotEscape(t.type)}" [label="${dotEscape(label)}"]`)
  }
  for (const t of types.value) {
    for (const r of t.rels) {
      if (!r.inDoc) continue
      L.push(`  "${dotEscape(t.type)}" -> "${dotEscape(r.targetType)}" [label="${dotEscape(r.name)}"]`)
    }
  }
  L.push('}')
  return L.join('\n')
}

function exportJson(): string {
  const data = types.value.map((t) => ({
    type: t.type,
    count: t.count,
    attributes: t.attributes,
    relationships: t.rels.map((r) => ({
      name: r.name,
      targetType: r.targetType,
      cardinality: r.many ? 'to-many' : 'to-one',
      inDocument: r.inDoc,
      ...(r.relatedUrl ? { relatedUrl: r.relatedUrl } : {}),
    })),
    incoming: t.incoming.map((i) => ({ fromType: i.fromType, rel: i.rel })),
  }))
  return JSON.stringify({ types: data }, null, 2)
}

function generate(format: ExportId): string {
  if (format === 'mermaid') return exportMermaid()
  if (format === 'dot') return exportDot()
  return exportJson()
}

async function copyExport(format: ExportId) {
  if (!types.value.length) return
  if (await copyToClipboard(generate(format))) {
    copied.value = true
    setTimeout(() => (copied.value = false), 1500)
  }
}

// --- Compare schemas ---
const compareOpen = ref(false)
const compareDoc = ref<JsonApiDocument | null>(null)
const compareFilter = ref('')
const pasteBody = ref('')
const pasteError = ref('')

const compareOptions = computed(() => {
  const q = compareFilter.value.trim().toLowerCase()
  return store.requests
    .map((r) => {
      const p = tryParseJson(r.responseBody)
      return {
        id: r.id,
        method: r.method,
        url: r.url,
        startedAt: r.startedAt,
        doc: p.ok && isJsonApi(p.value) ? (p.value as JsonApiDocument) : null,
      }
    })
    .filter((o) => o.doc != null)
    .filter((o) => {
      if (!q) return true
      return o.method.toLowerCase().includes(q) || o.url.toLowerCase().includes(q)
    })
})

const diff = computed<TypeDiff[] | null>(() => {
  if (!compareDoc.value) return null
  return diffSchemas(types.value, buildSchema(compareDoc.value))
})

function formatTime(startedAt: number): string {
  const d = new Date(startedAt)
  return d.toLocaleString('ru-RU', {
    day: '2-digit',
    month: '2-digit',
    year: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function pickCompare(id: string) {
  const o = compareOptions.value.find((x) => x.id === id)
  if (o && o.doc) {
    compareDoc.value = o.doc
    compareOpen.value = false
  }
}

function comparePaste() {
  const p = tryParseJson(pasteBody.value)
  if (p.ok && isJsonApi(p.value)) {
    compareDoc.value = p.value as JsonApiDocument
    compareOpen.value = false
    pasteError.value = ''
  } else {
    pasteError.value = 'Невалидный JSON:API документ'
  }
}

function closeCompare() {
  compareDoc.value = null
}

function statusLabel(s: string): string {
  if (s === 'added') return 'добавлен'
  if (s === 'removed') return 'удалён'
  return 'изменён'
}

</script>

<template>
  <div class="schema">
    <div class="schema-head">
      <template v-if="searchVisible">
        <Input
          ref="searchInputRef"
          v-model="query"
          mono
          class="flex-1 min-w-0"
          placeholder="Поиск по типам, полям, связям…"
          spellcheck="false"
          @keydown.esc="closeSearch"
        />
        <span v-if="query.trim()" class="search-count">{{ filteredTypes.length }} найдено</span>
        <IconButton hint="Закрыть (Esc)" @click="closeSearch"><Icon name="xmark" :size="14" /></IconButton>
      </template>
      <template v-else>
        <span v-if="types.length && !diff" class="summary">{{ types.length }} типов · {{ all.length }} ресурсов</span>

        <span class="head-spacer"></span>

        <Button size="sm" @click="openSearch"><span>Поиск</span><kbd class="keycap">{{ searchShortcut }}</kbd></Button>

        <Button size="sm" @click="compareOpen = true">
          <Icon name="compare" :size="14" />
          <span>Сравнить</span>
        </Button>
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <Button size="sm" :disabled="!types.length">
              <Icon v-if="copied" name="check" :size="12" />
              <span>{{ copied ? 'Скопировано' : 'Экспорт' }}</span>
              <svg viewBox="0 0 10 6" width="10" height="6" fill="none" aria-hidden="true"><path d="M1.5 1.5L5 5L8.5 1.5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent class="export-menu" :side-offset="4">
            <DropdownMenuItem v-for="f in EXPORT_FORMATS" :key="f.id" class="export-item" @select="copyExport(f.id)">
              {{ f.label }}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </template>
    </div>

    <div class="schema-body">
      <template v-if="diff">
        <div class="diff-head">
          <span class="diff-title">Сравнение схем</span>
          <Button @click="closeCompare">Закрыть</Button>
        </div>
        <div v-if="diff.length === 0" class="empty">Схемы идентичны</div>
        <div v-else class="diff-list">
          <div v-for="d in diff" :key="d.type" class="diff-type">
            <div class="diff-type-head">
              <span class="diff-badge" :class="d.status">{{ statusLabel(d.status) }}</span>
              <span class="diff-type-name">{{ d.label }}</span>
            </div>
            <div class="diff-lines">
              <div v-for="a in d.addedAttrs" :key="'aa' + a" class="diff-line added">+ {{ a }}</div>
              <div v-for="a in d.removedAttrs" :key="'ra' + a" class="diff-line removed">− {{ a }}</div>
              <div v-for="a in d.addedRels" :key="'ar' + a" class="diff-line added">+ {{ a }}</div>
              <div v-for="a in d.removedRels" :key="'rr' + a" class="diff-line removed">− {{ a }}</div>
            </div>
          </div>
        </div>
      </template>

      <template v-else>
        <div v-if="filteredTypes.length === 0" class="empty">
          {{ types.length === 0 ? 'Нет данных для карты' : 'Ничего не найдено' }}
        </div>

        <div v-else class="cards">
          <section
            v-for="t in filteredTypes"
            :key="t.type"
            :data-type="t.type"
            class="type-card"
            :class="{ highlight: highlightType === t.type }"
          >
            <button class="type-head" @click="toggleType(t.type)">
              <span class="caret" :class="{ open: expandedTypes.has(t.type) }">
                <Icon name="chevron-right" :size="10" />
              </span>
              <span class="type-name">{{ t.label }}</span>
              <span class="type-count">{{ t.count }}</span>
            </button>

            <div class="type-body">
              <div v-if="t.attributes.length" class="type-attrs">
                <span v-for="a in t.attributes" :key="a" class="attr-chip mono">{{ a }}</span>
              </div>

              <div v-if="t.rels.length" class="type-rels">
                <div v-for="r in t.rels" :key="r.name + r.targetType" class="type-rel">
                  <span class="rel-name">{{ r.name }}</span>
                  <span class="rel-card">{{ r.many ? '1:N' : '1:1' }}</span>
                  <span class="rel-arrow"><Icon name="arrow-right" :size="12" /></span>
                  <button v-if="r.inDoc" class="rel-target in-doc" @click="goToType(r.targetType)">
                    {{ humanize(r.targetType) }}
                  </button>
                  <button v-else-if="r.relatedUrl" class="rel-target missing" @click="emit('fetch', r.relatedUrl)">
                    <span>{{ humanize(r.targetType) }}</span>
                    <Icon name="arrow-up-right" :size="12" />
                  </button>
                  <span v-else class="rel-target ghost">{{ humanize(r.targetType) }}</span>
                </div>
              </div>

              <div v-if="t.incoming.length" class="type-incoming">
                <div class="incoming-title">Связан из</div>
                <div class="incoming-list">
                  <button
                    v-for="inc in t.incoming"
                    :key="inc.fromType + inc.rel"
                    class="incoming-chip"
                    @click="goToType(inc.fromType)"
                  >
                    <Icon name="arrow-left" :size="12" />
                    <span>{{ humanize(inc.fromType) }}</span>
                    <span class="incoming-rel">{{ inc.rel }}</span>
                  </button>
                </div>
              </div>

              <div v-if="expandedTypes.has(t.type)" class="type-instances">
                <div class="instances-title">Экземпляры</div>
                <button
                  v-for="res in instancesOf(t.type)"
                  :key="resourceKey(res.type, res.id)"
                  :id="'inst-' + resourceKey(res.type, res.id)"
                  class="instance"
                  :class="{ highlighted: highlightInstance === resourceKey(res.type, res.id) }"
                  @click="emit('select', resourceKey(res.type, res.id))"
                >
                  {{ instanceLabel(res) }}
                </button>
              </div>
            </div>
          </section>
        </div>
      </template>
    </div>
  </div>

  <Dialog :open="compareOpen" class="compare-modal" @update:open="compareOpen = false">
      <div class="compare-modal-head">
        <DialogTitle class="compare-modal-title">Сравнить схему с…</DialogTitle>
        <IconButton @click="compareOpen = false"><Icon name="xmark" :size="14" /></IconButton>
      </div>

      <div class="compare-search">
        <Input v-model="compareFilter" size="sm" mono class="w-full" placeholder="Поиск по методу, URL…" spellcheck="false" />
      </div>

      <div class="compare-modal-body">
        <div v-if="compareOptions.length === 0" class="compare-empty">Нет JSON:API ответов в истории</div>
        <button v-for="o in compareOptions" :key="o.id" class="compare-item" @click="pickCompare(o.id)">
          <span class="compare-method">{{ o.method }}</span>
          <span class="compare-url mono">{{ o.url }}</span>
          <span class="compare-time">{{ formatTime(o.startedAt) }}</span>
        </button>
      </div>

      <div class="compare-paste">
        <div class="compare-paste-label">Или вставьте JSON</div>
        <textarea v-model="pasteBody" class="compare-textarea mono" placeholder="{ … JSON:API документ … }" spellcheck="false"></textarea>
        <div v-if="pasteError" class="compare-error">{{ pasteError }}</div>
        <Button variant="primary" :disabled="!pasteBody.trim()" @click="comparePaste">Сравнить с этим JSON</Button>
      </div>
  </Dialog>
</template>

<style scoped>
@reference "../../style.css";

/* Fills the tab's column and scrolls its body only, so the head with поиск,
   сравнение и экспорт stays put — the same rule as every other tab. */
.schema {
  @apply flex flex-col h-full min-h-0;
}

.schema-head {
  @apply flex-none flex items-center gap-2 py-2 px-3 border-b border-border bg-bg-panel;
}

.schema-body {
  @apply flex-1 min-h-0 overflow-auto pt-3.5 px-4 pb-5;
}


.export-item {
  @apply overflow-hidden text-ellipsis whitespace-nowrap;
}

.compare-empty {
  @apply p-5 text-center text-text-tertiary text-[13px];
}

.summary {
  @apply text-xs text-text-tertiary whitespace-nowrap;
}

.empty {
  @apply text-text-tertiary text-[13px] py-6 text-center;
}

.cards {
  @apply flex flex-col gap-2.5;
}

.type-card {
  @apply rounded-xl pt-2.5 px-3.5 pb-3.5;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

.type-card.highlight {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.type-head {
  @apply flex items-center gap-[7px] w-full border-none bg-transparent py-1 px-0 cursor-pointer text-left;
  font: inherit;
  --wails-draggable: no-drag;
}

.caret {
  @apply inline-flex items-center justify-center flex-none w-3 h-3 text-text-secondary;
  transition: transform 0.12s ease;
}

.caret.open {
  @apply rotate-90;
}

.type-name {
  @apply text-sm font-semibold text-text;
}

.type-count {
  @apply text-[11px] text-text-tertiary rounded-lg py-px px-[7px];
  background: var(--bg-inset);
  border: 1px solid var(--border);
  font-variant-numeric: tabular-nums;
}

.type-body {
  @apply pl-[19px];
}

.type-attrs {
  @apply flex flex-wrap gap-[5px] mt-2;
}

.attr-chip {
  @apply text-[11px] text-text-secondary rounded-md py-0.5 px-[7px];
  background: var(--bg-inset);
  border: 1px solid var(--border);
}

.type-rels {
  @apply flex flex-col gap-[7px] mt-2.5 pt-2.5 border-t border-border;
}

.type-rel {
  @apply flex items-center gap-2 text-[13px] flex-wrap;
}

.rel-name {
  @apply font-medium text-text;
}

.rel-card {
  @apply text-[10px] text-text-tertiary rounded-sm py-px px-[5px];
  border: 1px solid var(--border);
  font-family: var(--mono);
}

.rel-arrow {
  @apply text-text-tertiary;
}

.rel-target {
  @apply inline-flex items-center gap-1 border-none bg-transparent py-0.5 px-2 rounded-md text-[13px] cursor-pointer;
  font: inherit;
  --wails-draggable: no-drag;
}

.rel-target.in-doc {
  @apply text-accent bg-accent-soft;
}

.rel-target.in-doc:hover {
  @apply underline;
}

.rel-target.missing {
  color: var(--orange);
  border: 1px dashed color-mix(in srgb, var(--orange) 55%, transparent);
}

.rel-target.missing:hover {
  @apply border-solid;
}

.rel-target.ghost {
  @apply text-text-tertiary cursor-default;
}

.type-incoming {
  @apply mt-2.5 pt-2.5 border-t border-border;
}

.incoming-title {
  @apply text-[11px] uppercase tracking-wider text-text-tertiary mb-1;
}

.incoming-list {
  @apply flex flex-wrap gap-[5px];
}

.incoming-chip {
  @apply inline-flex items-center gap-[5px] text-text-secondary text-[11px] py-0.5 px-2 rounded-md cursor-pointer;
  border: 1px solid var(--border);
  background: var(--bg-inset);
  font: inherit;
  --wails-draggable: no-drag;
}

.incoming-chip:hover {
  border-color: var(--accent);
  @apply text-text;
}

.incoming-rel {
  @apply text-text-tertiary;
  font-family: var(--mono);
}

.type-instances {
  @apply mt-2.5 pt-2.5 border-t border-border flex flex-col gap-1;
}

.instances-title {
  @apply text-[11px] uppercase tracking-wider text-text-tertiary mb-0.5;
}

.instance {
  @apply border-none bg-transparent text-left text-xs text-text-secondary py-1 px-2 rounded-md cursor-pointer;
  font: inherit;
  --wails-draggable: no-drag;
}

.instance:hover {
  @apply bg-bg-hover text-accent;
}

.instance.highlighted {
  @apply text-accent bg-accent-soft;
}

/* --- Schema diff --- */
.diff-head {
  @apply flex items-center justify-between mb-3;
}

.diff-title {
  @apply text-[13px] font-semibold;
}

.diff-list {
  @apply flex flex-col gap-2.5;
}

.diff-type {
  @apply rounded-xl py-3 px-3.5;
  background: var(--bg-panel);
  border: 1px solid var(--border);
}

.diff-type-head {
  @apply flex items-center gap-2 mb-1.5;
}

.diff-badge {
  @apply text-[10px] font-semibold uppercase tracking-wide py-px px-[7px] rounded-full;
}

.diff-badge.added {
  @apply text-green bg-green-soft;
}

.diff-badge.removed {
  @apply text-red bg-red-soft;
}

.diff-badge.changed {
  color: var(--orange);
  background: color-mix(in srgb, var(--orange) 14%, transparent);
}

.diff-type-name {
  @apply font-semibold text-[13px];
}

.diff-lines {
  @apply flex flex-col gap-[3px] pl-2;
}

.diff-line {
  @apply text-xs;
  font-family: var(--mono);
}

.diff-line.added {
  @apply text-green;
}

.diff-line.removed {
  @apply text-red;
}


.compare-modal-head {
  @apply flex items-center justify-between py-3 px-4 border-b border-border;
}

.compare-modal-title {
  @apply text-sm font-semibold;
}

.compare-modal-body {
  @apply flex-1 min-h-0 p-2 overflow-auto flex flex-col gap-0.5;
}

.compare-item {
  @apply flex items-center gap-2.5 py-2 px-2.5 border-none rounded-lg bg-transparent cursor-pointer text-left;
  font: inherit;
  --wails-draggable: no-drag;
}

.compare-item:hover {
  @apply bg-bg-hover;
}

.compare-method {
  @apply text-[11px] font-semibold text-accent flex-none min-w-11;
}

.compare-url {
  @apply flex-1 min-w-0 text-xs text-text overflow-hidden text-ellipsis whitespace-nowrap;
}

.compare-time {
  @apply flex-none text-[11px] text-text-tertiary whitespace-nowrap;
}

.compare-search {
  @apply py-2 px-3 border-b border-border;
}

.compare-paste {
  @apply pt-2.5 px-3 pb-3 border-t border-border flex flex-col gap-1.5;
}

.compare-paste-label {
  @apply text-xs text-text-secondary;
}

.compare-textarea {
  @apply w-full min-h-16 resize-y py-1.5 px-2 rounded-md text-xs outline-none select-text;
  border: 1px solid var(--border);
  background: var(--bg-inset);
  color: var(--text);
}

.compare-textarea:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.compare-error {
  @apply text-xs text-red;
}
</style>
