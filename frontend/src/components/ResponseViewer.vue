<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { RequestRecord } from '../lib/types'
import {
  tryParseJson,
  prettyJson,
  highlightJson,
  formatBytes,
  formatDuration,
  statusClass,
} from '../lib/json'
import { isJsonApi, type JsonApiDocument } from '../lib/jsonapi'
import JsonApiTree from './JsonApiTree.vue'
import JsonTree from './JsonTree.vue'
import RelationshipMap from './RelationshipMap.vue'
import { Fetch, Analyze } from '../../wailsjs/go/main/App'
import type { jsonapi } from '../../wailsjs/go/models'
import { useRequestsStore } from '../stores/requests'

const props = defineProps<{ record: RequestRecord }>()

const store = useRequestsStore()

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

function escapeRegex(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function findMatches(text: string, query: string): { start: number; end: number }[] {
  if (!query) return []
  const re = new RegExp(escapeRegex(query), 'gi')
  const out: { start: number; end: number }[] = []
  let m: RegExpExecArray | null
  while ((m = re.exec(text)) !== null) {
    out.push({ start: m.index, end: m.index + m[0].length })
    if (m.index === re.lastIndex) re.lastIndex++
  }
  return out
}

function renderWithMarks(
  text: string,
  matches: { start: number; end: number }[],
  current: number
): string {
  let html = ''
  let last = 0
  matches.forEach((m, i) => {
    html += escapeHtml(text.slice(last, m.start))
    const cls = i === current ? 'search-match current' : 'search-match'
    html += `<mark class="${cls}">${escapeHtml(text.slice(m.start, m.end))}</mark>`
    last = m.end
  })
  html += escapeHtml(text.slice(last))
  return html
}

type Tab = 'body' | 'map' | 'raw' | 'headers' | 'request'
const activeTab = ref<Tab>('body')

const parsed = computed(() => tryParseJson(props.record.responseBody))
const isJson = computed(() => parsed.value.ok)
const jsonValue = computed(() => parsed.value.value)
const isJsonApiDoc = computed(() => isJson.value && isJsonApi(jsonValue.value))
const doc = computed<JsonApiDocument | null>(() =>
  isJsonApiDoc.value ? (jsonValue.value as JsonApiDocument) : null
)

const graph = ref<jsonapi.Graph | null>(null)
const mapLoaded = ref(false)
const highlightKey = ref<string | null>(null)

async function loadMap() {
  if (mapLoaded.value || !isJsonApiDoc.value) return
  mapLoaded.value = true
  try {
    const analysis = await Analyze(props.record.responseBody)
    graph.value = analysis.graph ?? null
  } catch {
    graph.value = null
  }
}

watch(activeTab, (t) => {
  if (t === 'map') loadMap()
})

watch(
  () => props.record.id,
  () => {
    activeTab.value = 'body'
    graph.value = null
    mapLoaded.value = false
    highlightKey.value = null
  },
  { immediate: true }
)

function onTreeFetch(url: string) {
  follow(url)
}

function onMapSelect(key: string) {
  highlightKey.value = key
  activeTab.value = 'body'
}

function onMapFetch(url: string) {
  follow(url)
}

async function follow(url: string) {
  const headers = props.record.requestHeaders
  // Switch to the request view immediately and clear it, so the user isn't
  // left staring at the stale response while the new request runs.
  store.activeView = 'request'
  store.loading = true
  store.selectedId = null
  try {
    const res = await Fetch(url, headers)
    store.add({
      method: 'GET',
      url,
      requestHeaders: headers,
      requestBody: '',
      status: res.status,
      statusText: res.statusText,
      responseHeaders: res.headers,
      responseBody: res.body,
      durationMs: res.durationMs,
      contentType: res.contentType,
      error: res.error,
      source: 'manual',
    })
  } finally {
    store.loading = false
  }
}

const bodySize = computed(() => new Blob([props.record.responseBody]).size)

const responseHeaderEntries = computed(() => Object.entries(props.record.responseHeaders ?? {}))
const requestHeaderEntries = computed(() => Object.entries(props.record.requestHeaders ?? {}))

const hasPrev = computed(() => {
  const idx = store.requests.findIndex((r) => r.id === props.record.id)
  return idx >= 0 && idx < store.requests.length - 1
})

function goBack() {
  const idx = store.requests.findIndex((r) => r.id === props.record.id)
  if (idx >= 0 && idx < store.requests.length - 1) {
    store.select(store.requests[idx + 1].id)
  }
}

const prettyRaw = computed(() => (isJson.value ? prettyJson(jsonValue.value) : props.record.responseBody))
const rawHtml = computed(() => highlightJson(prettyRaw.value))

// --- Raw search (Cmd/Ctrl+F) ---
const rawSearchQuery = ref('')
const rawSearchVisible = ref(false)
const rawCurrentMatch = ref(0)
const searchInputRef = ref<HTMLInputElement | null>(null)

const rawMatches = computed(() => findMatches(prettyRaw.value, rawSearchQuery.value))

const rawRender = computed(() => {
  if (!rawSearchVisible.value || rawSearchQuery.value === '') {
    return rawHtml.value
  }
  return renderWithMarks(prettyRaw.value, rawMatches.value, rawCurrentMatch.value)
})

watch(rawMatches, (m) => {
  if (rawCurrentMatch.value >= m.length) rawCurrentMatch.value = 0
})

function nextMatch() {
  if (rawMatches.value.length === 0) return
  rawCurrentMatch.value = (rawCurrentMatch.value + 1) % rawMatches.value.length
  scrollToCurrentMatch()
}

function prevMatch() {
  if (rawMatches.value.length === 0) return
  rawCurrentMatch.value = (rawCurrentMatch.value - 1 + rawMatches.value.length) % rawMatches.value.length
  scrollToCurrentMatch()
}

function onSearchEnter(e: KeyboardEvent) {
  e.preventDefault()
  if (e.shiftKey) prevMatch()
  else nextMatch()
}

function scrollToCurrentMatch() {
  nextTick(() => {
    document.querySelector('.search-match.current')?.scrollIntoView({ block: 'center' })
  })
}

function openSearch() {
  rawSearchVisible.value = true
  rawCurrentMatch.value = 0
  nextTick(() => searchInputRef.value?.focus())
}

function closeSearch() {
  rawSearchVisible.value = false
  rawSearchQuery.value = ''
  rawCurrentMatch.value = 0
}

function onWindowKeydown(e: KeyboardEvent) {
  if (activeTab.value !== 'raw') return
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'f') {
    e.preventDefault()
    openSearch()
  } else if (e.key === 'Escape' && rawSearchVisible.value) {
    closeSearch()
  }
}

onMounted(() => window.addEventListener('keydown', onWindowKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onWindowKeydown))
</script>

<template>
  <div class="resp">
    <div class="resp-bar">
      <button v-if="hasPrev" class="btn icon-btn" title="Назад" @click="goBack">←</button>
      <span class="badge badge-method">{{ record.method }}</span>
      <span class="badge" :class="statusClass(record.status)">{{ record.status }}</span>
      <span class="resp-url mono" :title="record.url">{{ record.url }}</span>
      <span class="resp-meta">{{ formatDuration(record.durationMs) }} · {{ formatBytes(bodySize) }}</span>
    </div>

    <div v-if="record.error" class="resp-error">Ошибка: {{ record.error }}</div>

    <div class="tabs">
      <button class="tab" :class="{ active: activeTab === 'body' }" @click="activeTab = 'body'">Тело</button>
      <button v-if="isJsonApiDoc" class="tab" :class="{ active: activeTab === 'map' }" @click="activeTab = 'map'">
        Карта
      </button>
      <button class="tab" :class="{ active: activeTab === 'raw' }" @click="activeTab = 'raw'">Raw</button>
      <button class="tab" :class="{ active: activeTab === 'headers' }" @click="activeTab = 'headers'">Заголовки</button>
      <button class="tab" :class="{ active: activeTab === 'request' }" @click="activeTab = 'request'">Запрос</button>
    </div>

    <div class="resp-content">
      <template v-if="activeTab === 'body'">
        <JsonApiTree v-if="doc" :doc="doc" :highlight-key="highlightKey" @fetch="onTreeFetch" />
        <div v-else-if="isJson" class="jt-wrap"><JsonTree :value="jsonValue" /></div>
        <pre v-else class="code resp-pad">{{ record.responseBody }}</pre>
      </template>

      <template v-else-if="activeTab === 'map'">
        <RelationshipMap :graph="graph" @select="onMapSelect" @fetch="onMapFetch" />
      </template>

      <template v-else-if="activeTab === 'raw'">
        <div v-if="rawSearchVisible" class="search-bar">
          <input
            ref="searchInputRef"
            v-model="rawSearchQuery"
            class="input search-input mono"
            placeholder="Поиск…"
            spellcheck="false"
            @keydown.enter="onSearchEnter"
            @keydown.esc="closeSearch"
          />
          <span class="search-count">
            {{ rawMatches.length ? `${rawCurrentMatch + 1} / ${rawMatches.length}` : 'нет совпадений' }}
          </span>
          <button class="btn icon-btn" title="Предыдущее (Shift+Enter)" @click="prevMatch">↑</button>
          <button class="btn icon-btn" title="Следующее (Enter)" @click="nextMatch">↓</button>
          <button class="btn icon-btn" title="Закрыть (Esc)" @click="closeSearch">×</button>
        </div>
        <pre class="code resp-pad" v-html="rawRender"></pre>
      </template>

      <template v-else-if="activeTab === 'headers'">
        <table class="kv-table">
          <tbody>
            <tr v-for="[k, v] in responseHeaderEntries" :key="k">
              <td class="kv-key mono">{{ k }}</td>
              <td class="kv-val mono">{{ v }}</td>
            </tr>
            <tr v-if="responseHeaderEntries.length === 0">
              <td class="kv-key">Нет заголовков ответа</td>
            </tr>
          </tbody>
        </table>
      </template>

      <template v-else>
        <div class="resp-pad">
          <div class="kv-row"><span class="kv-label">Метод</span><span class="mono">{{ record.method }}</span></div>
          <div class="kv-row"><span class="kv-label">URL</span><span class="mono break">{{ record.url }}</span></div>
          <div class="ja-section-title" style="padding-left: 0">Заголовки запроса</div>
          <table class="kv-table">
            <tbody>
              <tr v-for="[k, v] in requestHeaderEntries" :key="k">
                <td class="kv-key mono">{{ k }}</td>
                <td class="kv-val mono">{{ v }}</td>
              </tr>
              <tr v-if="requestHeaderEntries.length === 0"><td class="kv-key">—</td></tr>
            </tbody>
          </table>
          <div v-if="record.requestBody" class="ja-section-title" style="padding-left: 0">Тело запроса</div>
          <pre v-if="record.requestBody" class="code" v-html="highlightJson(record.requestBody)"></pre>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.resp {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  background: var(--bg);
}

.resp-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border);
  background: var(--bg-panel);
}

.icon-btn {
  padding: 2px 8px;
}

.resp-url {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-secondary);
  font-size: 12px;
}

.resp-meta {
  color: var(--text-tertiary);
  font-size: 12px;
  white-space: nowrap;
}

.resp-error {
  padding: 10px 12px;
  color: var(--red);
  background: var(--red-soft);
  font-size: 12px;
}

.resp-content {
  flex: 1;
  min-height: 0;
  overflow: auto;
}

.resp-pad {
  padding: 12px 16px;
  margin: 0;
}

.jt-wrap {
  padding: 12px 16px;
}

.kv-table {
  border-collapse: collapse;
  width: 100%;
  font-size: 12px;
}

.kv-key {
  color: var(--text-secondary);
  padding: 4px 16px;
  vertical-align: top;
  white-space: nowrap;
}

.kv-val {
  color: var(--text);
  word-break: break-all;
  user-select: text;
  padding: 4px 0;
}

.kv-row {
  display: flex;
  gap: 12px;
  padding: 3px 0;
  font-size: 12px;
}

.kv-label {
  color: var(--text-secondary);
  min-width: 70px;
  flex: 0 0 70px;
}

.break {
  word-break: break-all;
}

.search-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border);
  background: var(--bg-panel);
  position: sticky;
  top: 0;
  z-index: 10;
}

.search-input {
  flex: 1;
  min-width: 0;
}

.search-count {
  font-size: 12px;
  color: var(--text-tertiary);
  white-space: nowrap;
}
</style>
