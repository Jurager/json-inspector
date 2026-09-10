<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Icon from './Icon.vue'
import type { JsonApiDocument, Resource } from '../lib/jsonapi'
import { dataResources, isJsonApi, resourceKey, resourceLabel } from '../lib/jsonapi'
import { buildSchema, diffSchemas, humanize, type TypeDiff } from '../lib/schema'
import { copyToClipboard } from '../lib/export'
import { tryParseJson } from '../lib/json'
import { useRequestsStore } from '../stores/requests'

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
const exportOpen = ref(false)
const exportWrap = ref<HTMLElement | null>(null)

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
  exportOpen.value = false
  if (!types.value.length) return
  if (await copyToClipboard(generate(format))) {
    copied.value = true
    setTimeout(() => (copied.value = false), 1500)
  }
}

// --- Compare schemas ---
const compareOpen = ref(false)
const compareId = ref<string | null>(null)

const compareOptions = computed(() =>
  store.requests
    .map((r) => {
      const p = tryParseJson(r.responseBody)
      return {
        id: r.id,
        method: r.method,
        url: r.url,
        doc: p.ok && isJsonApi(p.value) ? (p.value as JsonApiDocument) : null,
      }
    })
    .filter((o) => o.doc != null)
)

const compareDoc = computed(() => {
  if (!compareId.value) return null
  return compareOptions.value.find((o) => o.id === compareId.value)?.doc ?? null
})

const diff = computed<TypeDiff[] | null>(() => {
  if (!compareDoc.value) return null
  return diffSchemas(types.value, buildSchema(compareDoc.value))
})

function pickCompare(id: string) {
  compareId.value = id
  compareOpen.value = false
}

function closeCompare() {
  compareId.value = null
}

function statusLabel(s: string): string {
  if (s === 'added') return 'добавлен'
  if (s === 'removed') return 'удалён'
  return 'изменён'
}

function onDocClick(e: MouseEvent) {
  if (exportWrap.value && !exportWrap.value.contains(e.target as Node)) {
    exportOpen.value = false
  }
}

onMounted(() => document.addEventListener('click', onDocClick))
onBeforeUnmount(() => document.removeEventListener('click', onDocClick))
</script>

<template>
  <div class="schema">
    <div class="schema-head">
      <input
        v-model="query"
        class="search mono"
        placeholder="Поиск по типам, полям, связям…"
        spellcheck="false"
      />
      <div class="compare-wrap">
        <button class="export-btn" @click="compareOpen = true">
          <Icon name="compare" :size="14" />
          <span>Сравнить</span>
        </button>
      </div>
      <div ref="exportWrap" class="export-wrap">
        <button class="export-btn" :disabled="!types.length" @click="exportOpen = !exportOpen">
          <Icon v-if="copied" name="check" :size="12" />
          <span>{{ copied ? 'Скопировано' : 'Экспорт' }}</span>
          <svg viewBox="0 0 10 6" width="10" height="6" fill="none" aria-hidden="true"><path d="M1.5 1.5L5 5L8.5 1.5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
        </button>
        <div v-if="exportOpen" class="export-menu">
          <button v-for="f in EXPORT_FORMATS" :key="f.id" class="export-item" @click="copyExport(f.id)">
            {{ f.label }}
          </button>
        </div>
      </div>
      <span v-if="types.length && !diff" class="summary">{{ types.length }} типов · {{ all.length }} ресурсов</span>
    </div>

    <div class="schema-body">
      <template v-if="diff">
        <div class="diff-head">
          <span class="diff-title">Сравнение схем</span>
          <button class="btn" @click="closeCompare">Закрыть</button>
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
                <svg viewBox="0 0 8 12" width="8" height="12" fill="none" aria-hidden="true">
                  <path d="M1.5 1.5L6 6L1.5 10.5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
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

  <div v-if="compareOpen" class="compare-overlay" @click.self="compareOpen = false">
    <div class="compare-modal">
      <div class="compare-modal-head">
        <span class="compare-modal-title">Сравнить схему с…</span>
        <button class="btn icon-btn" @click="compareOpen = false"><Icon name="xmark" :size="14" /></button>
      </div>
      <div class="compare-modal-body">
        <div v-if="compareOptions.length === 0" class="compare-empty">Нет других JSON:API ответов в истории</div>
        <button v-for="o in compareOptions" :key="o.id" class="compare-item" @click="pickCompare(o.id)">
          <span class="compare-method">{{ o.method }}</span>
          <span class="compare-url mono">{{ o.url }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.schema {
  display: flex;
  flex-direction: column;
  min-height: 100%;
}

.schema-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border);
  background: var(--bg-panel);
}

.schema-body {
  padding: 14px 16px 20px;
}

.search {
  flex: 1;
  min-width: 0;
  padding: 6px 10px;
  border-radius: 7px;
  border: 1px solid var(--border);
  background: var(--bg-inset);
  color: var(--text);
  font-size: 12px;
  outline: none;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

.search:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.compare-wrap,
.export-wrap {
  position: relative;
  flex: 0 0 auto;
}

.export-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: 1px solid var(--border-strong);
  background: var(--bg-panel);
  color: var(--text);
  font-size: 12px;
  padding: 5px 10px;
  border-radius: 7px;
  cursor: pointer;
  box-shadow: var(--shadow-btn);
  --wails-draggable: no-drag;
}

.export-btn:hover {
  background: var(--bg-hover);
}

.export-btn:disabled {
  opacity: 0.5;
  cursor: default;
}

.export-menu {
  position: absolute;
  top: calc(100% + 4px);
  right: 0;
  z-index: 20;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: var(--shadow);
  padding: 4px;
  min-width: 220px;
  max-width: 320px;
  max-height: 300px;
  overflow: auto;
}

.export-item {
  display: block;
  width: 100%;
  text-align: left;
  padding: 6px 10px;
  border: none;
  background: transparent;
  color: var(--text);
  font-size: 12px;
  border-radius: 6px;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  --wails-draggable: no-drag;
}

.export-item:hover {
  background: var(--bg-hover);
}

.compare-empty {
  padding: 20px;
  text-align: center;
  color: var(--text-tertiary);
  font-size: 13px;
}

.summary {
  font-size: 12px;
  color: var(--text-tertiary);
  white-space: nowrap;
}

.empty {
  color: var(--text-tertiary);
  font-size: 13px;
  padding: 24px 0;
  text-align: center;
}

.cards {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.type-card {
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 10px 14px 14px;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

.type-card.highlight {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.type-head {
  display: flex;
  align-items: center;
  gap: 7px;
  width: 100%;
  border: none;
  background: transparent;
  padding: 4px 0;
  cursor: pointer;
  text-align: left;
  font: inherit;
  --wails-draggable: no-drag;
}

.caret {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 14px;
  width: 14px;
  height: 14px;
  font-size: 11px;
  line-height: 1;
  color: var(--text-secondary);
  transition: transform 0.12s ease;
}

.caret.open {
  transform: rotate(90deg);
}

.type-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
}

.type-count {
  font-size: 11px;
  color: var(--text-tertiary);
  background: var(--bg-inset);
  border: 1px solid var(--border);
  border-radius: 9px;
  padding: 1px 7px;
  font-variant-numeric: tabular-nums;
}

.type-body {
  padding-left: 19px;
}

.type-attrs {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  margin-top: 8px;
}

.attr-chip {
  font-size: 11px;
  color: var(--text-secondary);
  background: var(--bg-inset);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 2px 7px;
}

.type-rels {
  display: flex;
  flex-direction: column;
  gap: 7px;
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--border);
}

.type-rel {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  flex-wrap: wrap;
}

.rel-name {
  font-weight: 500;
  color: var(--text);
}

.rel-card {
  font-size: 10px;
  color: var(--text-tertiary);
  border: 1px solid var(--border);
  border-radius: 5px;
  padding: 1px 5px;
  font-family: var(--mono);
}

.rel-arrow {
  color: var(--text-tertiary);
}

.rel-target {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border: none;
  background: transparent;
  padding: 2px 8px;
  border-radius: 6px;
  font: inherit;
  font-size: 13px;
  cursor: pointer;
  --wails-draggable: no-drag;
}

.rel-target.in-doc {
  color: var(--accent);
  background: var(--accent-soft);
}

.rel-target.in-doc:hover {
  text-decoration: underline;
}

.rel-target.missing {
  color: var(--orange);
  border: 1px dashed color-mix(in srgb, var(--orange) 55%, transparent);
}

.rel-target.missing:hover {
  border-style: solid;
}

.rel-target.ghost {
  color: var(--text-tertiary);
  cursor: default;
}

.type-incoming {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--border);
}

.incoming-title {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-tertiary);
  margin-bottom: 4px;
}

.incoming-list {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}

.incoming-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  border: 1px solid var(--border);
  background: var(--bg-inset);
  color: var(--text-secondary);
  font: inherit;
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 6px;
  cursor: pointer;
  --wails-draggable: no-drag;
}

.incoming-chip:hover {
  border-color: var(--accent);
  color: var(--text);
}

.incoming-rel {
  color: var(--text-tertiary);
  font-family: var(--mono);
}

.type-instances {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.instances-title {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-tertiary);
  margin-bottom: 2px;
}

.instance {
  border: none;
  background: transparent;
  text-align: left;
  font: inherit;
  font-size: 12px;
  color: var(--text-secondary);
  padding: 4px 8px;
  border-radius: 6px;
  cursor: pointer;
  --wails-draggable: no-drag;
}

.instance:hover {
  background: var(--bg-hover);
  color: var(--accent);
}

.instance.highlighted {
  color: var(--accent);
  background: var(--accent-soft);
}

/* --- Schema diff --- */
.diff-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.diff-title {
  font-size: 13px;
  font-weight: 600;
}

.diff-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.diff-type {
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 12px 14px;
}

.diff-type-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.diff-badge {
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  padding: 1px 7px;
  border-radius: 9px;
}

.diff-badge.added {
  color: var(--green);
  background: var(--green-soft);
}

.diff-badge.removed {
  color: var(--red);
  background: var(--red-soft);
}

.diff-badge.changed {
  color: var(--orange);
  background: color-mix(in srgb, var(--orange) 14%, transparent);
}

.diff-type-name {
  font-weight: 600;
  font-size: 13px;
}

.diff-lines {
  display: flex;
  flex-direction: column;
  gap: 3px;
  padding-left: 8px;
}

.diff-line {
  font-size: 12px;
  font-family: var(--mono);
}

.diff-line.added {
  color: var(--green);
}

.diff-line.removed {
  color: var(--red);
}

.icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 2px 8px;
}

.compare-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 3000;
}

.compare-modal {
  width: 480px;
  max-width: 90%;
  max-height: 70vh;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: 14px;
  box-shadow: var(--shadow);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.compare-modal-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
}

.compare-modal-title {
  font-size: 14px;
  font-weight: 600;
}

.compare-modal-body {
  padding: 8px;
  overflow: auto;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.compare-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border: none;
  border-radius: 8px;
  background: transparent;
  cursor: pointer;
  text-align: left;
  font: inherit;
  --wails-draggable: no-drag;
}

.compare-item:hover {
  background: var(--bg-hover);
}

.compare-method {
  font-size: 11px;
  font-weight: 600;
  color: var(--accent);
  flex: 0 0 auto;
  min-width: 44px;
}

.compare-url {
  flex: 1;
  min-width: 0;
  font-size: 12px;
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
