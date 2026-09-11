<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Icon from './Icon.vue'
import type { RequestRecord } from '../lib/types'
import {
  tryParseJson,
  prettyJson,
  highlightJson,
  formatBytes,
  formatDuration,
  statusClass,
} from '../lib/json'
import { dataResources, href, isJsonApi, resourceMatchesQuery, type JsonApiDocument } from '../lib/jsonapi'
import JsonApiTree from './JsonApiTree.vue'
import JsonTree from './JsonTree.vue'
import SchemaMap from './SchemaMap.vue'
import NodeInspector from './NodeInspector.vue'
import CookiesTab from './CookiesTab.vue'
import TimingsTab from './TimingsTab.vue'
import { Fetch } from '../../wailsjs/go/main/App'
import { useRequestsStore } from '../stores/requests'
import { copyToClipboard, exportRequest, type ExportFormat } from '../lib/export'
import { shortcut } from '../lib/platform'

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

type Tab = 'body' | 'map' | 'raw' | 'headers' | 'cookies' | 'timings' | 'request'
const activeTab = ref<Tab>('body')

const parsed = computed(() => tryParseJson(props.record.responseBody))
const isJson = computed(() => parsed.value.ok)
const jsonValue = computed(() => parsed.value.value)
const isJsonApiDoc = computed(() => isJson.value && isJsonApi(jsonValue.value))
const doc = computed<JsonApiDocument | null>(() =>
  isJsonApiDoc.value ? (jsonValue.value as JsonApiDocument) : null
)

const pagination = computed(() => {
  const l = doc.value?.links ?? {}
  return {
    first: href(l.first),
    prev: href(l.prev),
    next: href(l.next),
    last: href(l.last),
  }
})

const hasPagination = computed(() => Boolean(pagination.value.prev || pagination.value.next))

// --- Тело search (Cmd/Ctrl+F) — lives in the same toolbar row as the
// pagination controls below, rather than a container of its own; JsonApiTree
// just filters by whatever query it's handed. ---
const bodyQuery = ref('')
const bodySearchVisible = ref(false)
const bodySearchInputRef = ref<HTMLInputElement | null>(null)

const bodyMatchCount = computed(() => {
  if (!doc.value) return 0
  const q = bodyQuery.value.trim().toLowerCase()
  if (!q) return 0
  const all = [...dataResources(doc.value), ...(doc.value.included ?? [])]
  return all.filter((r) => resourceMatchesQuery(r, q)).length
})

function openBodySearch() {
  bodySearchVisible.value = true
  nextTick(() => bodySearchInputRef.value?.focus())
}

function closeBodySearch() {
  bodySearchVisible.value = false
  bodyQuery.value = ''
}

const highlightKey = ref<string | null>(null)

watch(
  () => props.record.id,
  () => {
    activeTab.value = 'body'
    highlightKey.value = null
  },
  { immediate: true }
)

function onTreeFetch(url: string) {
  follow(url)
}

function onTreeSelect(key: string) {
  highlightKey.value = key
}

function onInspect(path: string) {
  store.setInspector({ path, open: true })
}

function toggleInspector() {
  store.setInspector({ open: !store.inspector.open })
}

function onMapSelect(key: string) {
  highlightKey.value = key
  activeTab.value = 'body'
}

function onMapFetch(url: string) {
  follow(url)
}

async function follow(url: string) {
  if (!url) return
  const headers = props.record.requestHeaders
  // Switch to the request view immediately and clear it, so the user isn't
  // left staring at the stale response while the new request runs.
  store.activeView = 'request'
  store.loading = true
  store.manualId = null
  try {
    const res = await Fetch(url, headers)
    if (res.cancelled) return
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
      dnsMs: res.dnsMs,
      connectMs: res.connectMs,
      tlsMs: res.tlsMs,
      waitMs: res.waitMs,
      downloadMs: res.downloadMs,
      source: 'manual',
    })
  } finally {
    store.loading = false
  }
}

const bodySize = computed(() => new Blob([props.record.responseBody]).size)

// --- Browser mode: host+path + read-only query params (step 5) ---
function hostPath(url: string): string {
  try {
    const u = new URL(url)
    return u.host + u.pathname
  } catch {
    return url
  }
}

const browserParams = computed(() => {
  const out: { name: string; value: string }[] = []
  try {
    const u = new URL(props.record.url)
    u.searchParams.forEach((value, name) => out.push({ name, value }))
  } catch {
    // invalid URL — no params to show
  }
  return out
})

const paramsOpen = ref(false)
const paramsWrapEl = ref<HTMLElement | null>(null)

const hasCookies = computed(() =>
  Object.keys(props.record.responseHeaders ?? {}).some((k) => k.toLowerCase() === 'set-cookie')
)

const responseHeaderEntries = computed(() => Object.entries(props.record.responseHeaders ?? {}))
const requestHeaderEntries = computed(() => Object.entries(props.record.requestHeaders ?? {}))

// "Назад" steps within the record's own source list (manual or browser),
// not the combined list — otherwise going back from a captured request could
// silently reassign the *manual* selection instead of the browser one.
const sourceList = computed(() => store.requests.filter((r) => r.source === props.record.source))

const hasPrev = computed(() => {
  const idx = sourceList.value.findIndex((r) => r.id === props.record.id)
  return idx >= 0 && idx < sourceList.value.length - 1
})

function goBack() {
  const idx = sourceList.value.findIndex((r) => r.id === props.record.id)
  if (idx >= 0 && idx < sourceList.value.length - 1) {
    const prev = sourceList.value[idx + 1]
    if (prev.source === 'browser') store.selectBrowser(prev.id)
    else store.selectManual(prev.id)
  }
}

// "Открыть в «Запросе»" — copies a read-only captured request into the editable
// draft and switches rails, without sending anything.
function openInRequest() {
  store.loadDraft(props.record)
  store.setOpenChip(null)
  store.activeView = 'request'
  store.requestFocusUrl()
}

const prettyRaw = computed(() => (isJson.value ? prettyJson(jsonValue.value) : props.record.responseBody))
const rawHtml = computed(() => highlightJson(prettyRaw.value))

// --- Raw search (Cmd/Ctrl+F) ---
const rawSearchQuery = ref('')
const rawSearchVisible = ref(false)
const rawCurrentMatch = ref(0)
const rawSearchInputRef = ref<HTMLInputElement | null>(null)
const searchShortcut = computed(() => shortcut('F'))

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

function openRawSearch() {
  rawSearchVisible.value = true
  rawCurrentMatch.value = 0
  nextTick(() => rawSearchInputRef.value?.focus())
}

function closeRawSearch() {
  rawSearchVisible.value = false
  rawSearchQuery.value = ''
  rawCurrentMatch.value = 0
}

const rawCopied = ref(false)

async function copyRaw() {
  if (await copyToClipboard(prettyRaw.value)) {
    rawCopied.value = true
    setTimeout(() => (rawCopied.value = false), 1500)
  }
}

const headersCopied = ref(false)

async function copyHeaders() {
  const text = responseHeaderEntries.value.map(([k, v]) => `${k}: ${v}`).join('\n')
  if (await copyToClipboard(text)) {
    headersCopied.value = true
    setTimeout(() => (headersCopied.value = false), 1500)
  }
}

function onWindowKeydown(e: KeyboardEvent) {
  if (e.altKey && (e.key === 'i' || e.key === 'I')) {
    e.preventDefault()
    toggleInspector()
    return
  }
  if (activeTab.value === 'raw') {
    if ((e.metaKey || e.ctrlKey) && e.code === 'KeyF') {
      e.preventDefault()
      openRawSearch()
    } else if (e.key === 'Escape' && rawSearchVisible.value) {
      closeRawSearch()
    }
  } else if (activeTab.value === 'body' && doc.value) {
    if ((e.metaKey || e.ctrlKey) && e.code === 'KeyF') {
      e.preventDefault()
      openBodySearch()
    } else if (e.key === 'Escape' && bodySearchVisible.value) {
      closeBodySearch()
    }
  }
}

onMounted(() => window.addEventListener('keydown', onWindowKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onWindowKeydown))

// --- Copy request (Запрос tab) ---
const COPY_FORMATS: { id: ExportFormat; label: string }[] = [
  { id: 'curl', label: 'cURL' },
  { id: 'fetch', label: 'fetch (JS)' },
  { id: 'wget', label: 'wget' },
  { id: 'httpie', label: 'HTTPie' },
  { id: 'powershell', label: 'PowerShell' },
]

const copyMenuOpen = ref(false)
const copied = ref(false)
const copyWrapEl = ref<HTMLElement | null>(null)

async function copyAs(format: ExportFormat) {
  copyMenuOpen.value = false
  const text = exportRequest(
    format,
    props.record.method,
    props.record.url,
    props.record.requestHeaders,
    props.record.requestBody
  )
  if (await copyToClipboard(text)) {
    copied.value = true
    setTimeout(() => (copied.value = false), 1500)
  }
}

function onDocClick(e: MouseEvent) {
  if (copyWrapEl.value && !copyWrapEl.value.contains(e.target as Node)) {
    copyMenuOpen.value = false
  }
  if (paramsWrapEl.value && !paramsWrapEl.value.contains(e.target as Node)) {
    paramsOpen.value = false
  }
}

onMounted(() => document.addEventListener('click', onDocClick))
onBeforeUnmount(() => document.removeEventListener('click', onDocClick))
</script>

<template>
  <div class="resp">
    <div class="resp-bar">
      <button v-if="hasPrev" class="btn icon-btn" title="Назад" @click="goBack"><Icon name="chevron-left" :size="14" /></button>
      <span class="badge badge-method">{{ record.method }}</span>
      <span class="badge" :class="statusClass(record.status)">{{ record.status }}</span>

      <!-- URL is shown only for captured requests: for manual ones it already
           sits in the command line, so repeating it here would be noise. -->
      <span v-if="record.source === 'browser'" class="resp-url mono" :title="record.url">{{ hostPath(record.url) }}</span>
      <div v-if="record.source === 'browser' && browserParams.length" ref="paramsWrapEl" class="params-row">
        <button class="resp-action" @click="paramsOpen = !paramsOpen">Параметры {{ browserParams.length }}</button>
        <div v-if="paramsOpen" class="menu params-menu">
          <div v-for="p in browserParams" :key="p.name" class="params-item">
            <span class="params-name mono">{{ p.name }}</span>
            <span class="params-value mono">{{ p.value }}</span>
          </div>
        </div>
      </div>
      <template v-else>
        <span class="divider"></span>
        <span class="resp-meta">{{ formatDuration(record.durationMs) }}</span>
        <span class="divider"></span>
        <span class="resp-meta">{{ formatBytes(bodySize) }}</span>
        <span class="divider"></span>
        <span v-if="record.contentType" class="resp-meta truncate max-w-[240px]">{{ record.contentType }}</span>
      </template>

      <span class="resp-spacer"></span>

      <button class="resp-action" disabled title="Сравнение ответов — скоро">Сравнить</button>
      <div ref="copyWrapEl" class="copy-row">
        <button class="resp-action" @click="copyMenuOpen = !copyMenuOpen">
          <Icon v-if="copied" name="check" :size="12" />
          <span>{{ copied ? 'Скопировано' : 'Копировать' }}</span>
          <svg viewBox="0 0 10 6" width="10" height="6" fill="none" aria-hidden="true"><path d="M1.5 1.5L5 5L8.5 1.5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
        </button>
        <div v-if="copyMenuOpen" class="menu copy-menu">
          <button v-for="f in COPY_FORMATS" :key="f.id" class="menu-item" @click="copyAs(f.id)">
            {{ f.label }}
          </button>
        </div>
      </div>
      <button class="resp-action" title="Инспектор узла (⌥I)" @click="toggleInspector">Инспектор <kbd class="keycap">⌥I</kbd></button>
    </div>

    <div v-if="record.error" class="resp-error">Ошибка: {{ record.error }}</div>

    <div class="tabs">
      <button class="tab" :class="{ active: activeTab === 'body' }" @click="activeTab = 'body'">Тело</button>
      <button v-if="isJsonApiDoc" class="tab" :class="{ active: activeTab === 'map' }" @click="activeTab = 'map'">
        Карта
      </button>
      <button class="tab" :class="{ active: activeTab === 'raw' }" @click="activeTab = 'raw'">Raw</button>
      <button class="tab" :class="{ active: activeTab === 'headers' }" @click="activeTab = 'headers'">Заголовки</button>
      <button v-if="hasCookies" class="tab" :class="{ active: activeTab === 'cookies' }" @click="activeTab = 'cookies'">Cookies</button>
      <button class="tab" :class="{ active: activeTab === 'timings' }" @click="activeTab = 'timings'">Тайминги</button>
      <button class="tab" :class="{ active: activeTab === 'request' }" @click="activeTab = 'request'">Запрос</button>
    </div>

    <div class="resp-main">
      <div class="resp-content">
      <template v-if="activeTab === 'body'">
        <div v-if="doc" class="toolbar">
          <template v-if="bodySearchVisible">
            <input
              ref="bodySearchInputRef"
              v-model="bodyQuery"
              class="input mono flex-1 min-w-0"
              placeholder="Поиск по ресурсам, полям, значениям…"
              spellcheck="false"
              @keydown.esc="closeBodySearch"
            />
            <span v-if="bodyQuery.trim()" class="search-count">{{ bodyMatchCount }} найдено</span>
            <button class="btn icon-btn" title="Закрыть (Esc)" @click="closeBodySearch"><Icon name="xmark" :size="14" /></button>
          </template>
          <template v-else>
            <template v-if="hasPagination">
              <button class="btn icon-btn" :disabled="!pagination.first || !pagination.prev" title="Первая" @click="follow(pagination.first)"><Icon name="chevrons-left" :size="14" /></button>
              <button class="btn icon-btn" :disabled="!pagination.prev" title="Предыдущая" @click="follow(pagination.prev)"><Icon name="chevron-left" :size="14" /></button>
              <button class="btn icon-btn" :disabled="!pagination.next" title="Следующая" @click="follow(pagination.next)"><Icon name="chevron-right" :size="14" /></button>
              <button class="btn icon-btn" :disabled="!pagination.last || !pagination.next" title="Последняя" @click="follow(pagination.last)"><Icon name="chevrons-right" :size="14" /></button>
            </template>
            <span class="head-spacer"></span>
            <button v-if="record.source === 'browser'" class="btn open-in-request" @click="openInRequest">Открыть в «Запросе»</button>
            <button class="btn btn-inline" @click="openBodySearch"><span>Поиск</span><kbd class="keycap">{{ searchShortcut }}</kbd></button>
          </template>
        </div>
        <div v-else-if="record.source === 'browser'" class="toolbar">
          <span class="head-spacer"></span>
          <button class="btn open-in-request" @click="openInRequest">Открыть в «Запросе»</button>
        </div>
        <JsonApiTree v-if="doc" :doc="doc" :query="bodyQuery" :highlight-key="highlightKey" @select="onTreeSelect" @inspect="onInspect" @fetch="onTreeFetch" />
        <div v-else-if="isJson" class="jt-wrap"><JsonTree :value="jsonValue" /></div>
        <pre v-else class="code resp-pad">{{ record.responseBody }}</pre>
      </template>

      <template v-else-if="activeTab === 'map'">
        <SchemaMap :doc="doc" :highlight-key="highlightKey" @select="onMapSelect" @fetch="onMapFetch" />
      </template>

      <template v-else-if="activeTab === 'raw'">
        <div class="toolbar">
          <template v-if="rawSearchVisible">
            <input
              ref="rawSearchInputRef"
              v-model="rawSearchQuery"
              class="input mono flex-1 min-w-0"
              placeholder="Поиск…"
              spellcheck="false"
              @keydown.enter="onSearchEnter"
              @keydown.esc="closeRawSearch"
            />
            <span class="search-count">
              {{ rawMatches.length ? `${rawCurrentMatch + 1} / ${rawMatches.length}` : 'нет совпадений' }}
            </span>
            <button class="btn icon-btn" title="Предыдущее (Shift+Enter)" @click="prevMatch"><Icon name="chevron-up" :size="14" /></button>
            <button class="btn icon-btn" title="Следующее (Enter)" @click="nextMatch"><Icon name="chevron-down" :size="14" /></button>
            <button class="btn icon-btn" title="Закрыть (Esc)" @click="closeRawSearch"><Icon name="xmark" :size="14" /></button>
          </template>
          <template v-else>
            <button class="btn btn-inline" @click="copyRaw"><Icon v-if="rawCopied" name="check" :size="12" /><span>{{ rawCopied ? 'Скопировано' : 'Копировать' }}</span></button>
            <button class="btn btn-inline" @click="openRawSearch"><span>Поиск</span><kbd class="keycap">{{ searchShortcut }}</kbd></button>
          </template>
        </div>
        <pre class="code resp-pad" v-html="rawRender"></pre>
      </template>

      <template v-else-if="activeTab === 'headers'">
        <div class="toolbar">
          <button class="btn btn-inline" @click="copyHeaders"><Icon v-if="headersCopied" name="check" :size="12" /><span>{{ headersCopied ? 'Скопировано' : 'Копировать' }}</span></button>
        </div>
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

      <template v-else-if="activeTab === 'cookies'">
        <CookiesTab :headers="record.responseHeaders" />
      </template>

      <template v-else-if="activeTab === 'timings'">
        <TimingsTab :record="record" />
      </template>

      <template v-else>
        <div class="toolbar">
          <span class="request-caption">после подстановки переменных окружения</span>
        </div>
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
      <NodeInspector v-if="store.inspector.open" :doc="doc" @close="store.setInspector({ open: false })" @fetch="follow" />
    </div>
  </div>
</template>

<style scoped>
@reference "../style.css";

.resp {
  @apply flex flex-col h-full min-h-0 bg-bg;
}

.resp-bar {
  @apply flex items-center gap-2 h-12 px-3 border-b border-border bg-bg-panel;
}

.resp-url {
  @apply flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-text-secondary text-xs;
}

.resp-meta {
  @apply text-text-tertiary text-xs whitespace-nowrap;
}

.divider {
  @apply w-px h-3 bg-border flex-none;
}

.resp-spacer {
  @apply flex-1;
}

.resp-action {
  @apply inline-flex items-center gap-1.5 h-6 px-2.5 rounded-md text-[11.5px] flex-none;
  border: 1px solid var(--border-strong);
  background: var(--bg-panel);
  color: var(--text);
  cursor: pointer;
  --wails-draggable: no-drag;
}

.resp-action:hover {
  @apply bg-bg-hover;
}

.resp-action:disabled {
  @apply opacity-50 cursor-default;
}

.request-caption {
  @apply mr-auto text-[11.5px] text-text-tertiary;
}

.resp-error {
  @apply py-2.5 px-3 text-red bg-red-soft text-xs;
}

.resp-main {
  @apply flex flex-1 min-h-0;
}

.resp-content {
  @apply flex-1 min-h-0 overflow-auto;
}

.resp-pad {
  @apply py-3 px-4 m-0;
}

.jt-wrap {
  @apply py-3 px-4;
}

.kv-table {
  @apply border-collapse w-full text-xs;
}

.kv-key {
  @apply text-text-secondary py-1 px-4 align-top whitespace-nowrap;
}

.kv-val {
  @apply text-text break-all select-text py-1 px-0;
}

.kv-row {
  @apply flex gap-3 py-[3px] px-0 text-xs;
}

.kv-label {
  @apply text-text-secondary min-w-[70px] grow-0 shrink-0 basis-[70px];
}

.break {
  @apply break-all;
}

.copy-row {
  @apply relative;
}

.copy-menu {
  @apply absolute right-0;
  top: calc(100% + 4px);
}

.params-row {
  @apply relative flex-none;
}

.params-menu {
  @apply absolute right-0 w-[360px] max-h-64 overflow-auto;
  top: calc(100% + 4px);
}

.params-item {
  @apply flex gap-2 py-1 px-2;
}

.params-name {
  @apply flex-none min-w-[110px] text-[11.5px] text-text-secondary;
  font-family: var(--mono);
}

.params-value {
  @apply flex-1 min-w-0 text-[11.5px] text-text overflow-hidden text-ellipsis whitespace-nowrap;
  font-family: var(--mono);
}

.open-in-request {
  @apply text-accent;
}
</style>
