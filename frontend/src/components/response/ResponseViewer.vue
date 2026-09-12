<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Icon from '../ui/Icon.vue'
import { Button, IconButton } from '../ui/button'
import { Input } from '../ui/input'
import { Popover, PopoverTrigger, PopoverContent } from '../ui/popover'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../ui/tabs'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from '../ui/dropdown-menu'
import type { RequestRecord } from '../../lib/requestRecord'
import { tryParseJson, prettyJson, highlightJson } from '../../lib/json'
import { formatBytes, formatDuration, statusBadgeClass } from '../../lib/format'
import { dataResources, linkHref, isJsonApi, resourceMatchesQuery, type JsonApiDocument } from '../../lib/jsonapi'
import JsonApiTree from '../json/JsonApiTree.vue'
import TextViewerTab from './TextViewerTab.vue'
import SchemaMap from '../json/SchemaMap.vue'
import NodeInspector from '../json/NodeInspector.vue'
import CookiesTab from './CookiesTab.vue'
import TimingsTab from './TimingsTab.vue'
import TestsTab from './TestsTab.vue'
import { App as Backend } from '../../../bindings/json-inspector'
import { useRequestsStore } from '../../stores/requests'
import { useEnvironmentsStore } from '../../stores/environments'
import { copyToClipboard } from '../../lib/clipboard'
import { exportRequest, type ExportFormat } from '../../lib/export'
import { usePlatform } from '../../composables/usePlatform'
import { focusUrlField } from '../../composables/urlFocus'
import { normalizeHeaders } from '../../lib/headers'

const props = defineProps<{ record: RequestRecord }>()

const store = useRequestsStore()
const { shortcut } = usePlatform()
const envStore = useEnvironmentsStore()

type Tab = 'body' | 'map' | 'raw' | 'headers' | 'cookies' | 'timings' | 'tests' | 'request'
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
    first: linkHref(l.first),
    prev: linkHref(l.prev),
    next: linkHref(l.next),
    last: linkHref(l.last),
  }
})

const hasPagination = computed(() => Boolean(pagination.value.prev || pagination.value.next))

// Body search (Cmd/Ctrl+F) shares the pagination toolbar row rather than a container of its own.
const bodyQuery = ref('')
const bodySearchVisible = ref(false)
const bodySearchInputRef = ref<InstanceType<typeof Input> | null>(null)

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

function highlightResource(key: string) {
  highlightKey.value = key
}

function inspectNode(path: string) {
  store.setInspector({ path, open: true })
}

function toggleInspector() {
  store.setInspector({ open: !store.inspector.open })
}

function openResourceInBody(key: string) {
  highlightKey.value = key
  activeTab.value = 'body'
}

async function follow(url: string) {
  if (!url) return
  const headers = props.record.requestHeaders
  // Switch rails immediately so the user isn't left staring at the stale response.
  store.activeView = 'request'
  store.loading = true
  store.manualId = null
  try {
    const res = await Backend.Fetch(url, headers)
    // The binding types the Go pointer as nullable, but Go always returns a result — a type guard,
    // not a real branch.
    if (!res || res.cancelled) return
    store.addRequest({
      method: 'GET',
      url,
      requestHeaders: headers,
      requestBody: '',
      status: res.status,
      statusText: res.statusText,
      responseHeaders: normalizeHeaders(res.headers),
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

function hostPath(url: string): string {
  try {
    const u = new URL(url)
    return u.host + u.pathname
  } catch {
    return url
  }
}

const urlQueryParams = computed(() => {
  const out: { name: string; value: string }[] = []
  try {
    const u = new URL(props.record.url)
    u.searchParams.forEach((value, name) => out.push({ name, value }))
  } catch {
    // invalid URL — no params to show
  }
  return out
})


const hasCookies = computed(() =>
  Object.keys(props.record.responseHeaders ?? {}).some((k) => k.toLowerCase() === 'set-cookie')
)

const responseHeaderEntries = computed(() => Object.entries(props.record.responseHeaders ?? {}))
const requestHeaderEntries = computed(() => Object.entries(props.record.requestHeaders ?? {}))

// "Назад" walks the record's own source list, not the combined one — going back from a
// captured request must not silently reassign the *manual* selection instead.
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

function openInRequest() {
  store.loadDraft(props.record)
  store.setOpenChip(null)
  store.activeView = 'request'
  focusUrlField()
}

const prettyRaw = computed(() => (isJson.value ? prettyJson(jsonValue.value) : props.record.responseBody))
const searchShortcut = computed(() => shortcut('F'))

const bodyCopied = ref(false)

async function copyBody() {
  if (await copyToClipboard(prettyRaw.value)) {
    bodyCopied.value = true
    setTimeout(() => (bodyCopied.value = false), 1500)
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
  if (activeTab.value === 'body' && doc.value) {
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

const COPY_FORMATS: { id: ExportFormat; label: string }[] = [
  { id: 'curl', label: 'cURL' },
  { id: 'fetch', label: 'fetch (JS)' },
  { id: 'wget', label: 'wget' },
  { id: 'httpie', label: 'HTTPie' },
  { id: 'powershell', label: 'PowerShell' },
]

const copied = ref(false)

// A secret only ever leaves as dots (see lib/export.ts); `keepTokens` shares a snippet
// with the `{{tokens}}` intact instead of the values they stand for.
async function copyAs(format: ExportFormat, { keepTokens = false } = {}) {
  const text = exportRequest(format, props.record, { resolve: envStore.resolveVariable, keepTokens })
  if (await copyToClipboard(text)) {
    copied.value = true
    setTimeout(() => (copied.value = false), 1500)
  }
}

</script>

<template>
  <div class="resp">
    <div class="resp-bar">
      <IconButton v-if="hasPrev" variant="outline" hint="Назад" @click="goBack"><Icon name="chevron-left" :size="14" /></IconButton>
      <span class="badge badge-method">{{ record.method }}</span>
      <span class="badge" :class="statusBadgeClass(record.status)">{{ record.status }}</span>

      <!-- URL only for captured requests: a manual one already sits in the command line. -->
      <template v-if="record.source === 'browser'">
        <span class="resp-url mono" :title="record.url">{{ hostPath(record.url) }}</span>
        <Popover v-if="urlQueryParams.length">
          <PopoverTrigger as-child>
            <Button size="sm">Параметры {{ urlQueryParams.length }}</Button>
          </PopoverTrigger>
          <PopoverContent class="params-menu">
            <div v-for="p in urlQueryParams" :key="p.name" class="params-item">
              <span class="params-name mono">{{ p.name }}</span>
              <span class="params-value mono">{{ p.value }}</span>
            </div>
          </PopoverContent>
        </Popover>
      </template>
      <template v-else>
        <span class="divider"></span>
        <span class="resp-meta">{{ formatDuration(record.durationMs) }}</span>
        <span class="divider"></span>
        <span class="resp-meta">{{ formatBytes(bodySize) }}</span>
        <span class="divider"></span>
        <span v-if="record.contentType" class="resp-meta truncate max-w-[240px]">{{ record.contentType }}</span>
      </template>

      <span class="resp-spacer"></span>

      <!-- The handoff keeps "742 мс · 35,1 КБ" here, so duration/size must not
           vanish when a captured URL has query params. -->
      <span v-if="record.source === 'browser'" class="resp-meta">
        {{ formatDuration(record.durationMs) }} · {{ formatBytes(bodySize) }}
      </span>

      <Button size="sm" disabled title="Сравнение ответов — скоро">Сравнить</Button>
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <Button size="sm">
            <Icon v-if="copied" name="check" :size="12" />
            <span>{{ copied ? 'Скопировано' : 'Копировать' }}</span>
            <svg viewBox="0 0 10 6" width="10" height="6" fill="none" aria-hidden="true"><path d="M1.5 1.5L5 5L8.5 1.5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent :side-offset="4">
          <DropdownMenuItem v-for="f in COPY_FORMATS" :key="f.id" @select="copyAs(f.id)">
            {{ f.label }}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem @select="copyAs('curl', { keepTokens: true })">cURL с токенами</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <Button size="sm" title="Инспектор узла (⌥I)" @click="toggleInspector">Инспектор <kbd class="keycap">⌥I</kbd></Button>
    </div>

    <div v-if="record.error" class="resp-error">Ошибка: {{ record.error }}</div>

    <Tabs v-model="activeTab" class="resp-tabs">
      <TabsList>
        <TabsTrigger value="body">Тело</TabsTrigger>
        <TabsTrigger v-if="isJsonApiDoc" value="map">Карта</TabsTrigger>
        <TabsTrigger value="raw">Raw</TabsTrigger>
        <TabsTrigger value="headers">Заголовки</TabsTrigger>
        <TabsTrigger v-if="hasCookies" value="cookies">Cookies</TabsTrigger>
        <TabsTrigger value="timings">Тайминги</TabsTrigger>
        <TabsTrigger v-if="isJsonApiDoc" value="tests">Тесты</TabsTrigger>
        <TabsTrigger value="request">Запрос</TabsTrigger>
      </TabsList>

    <div class="resp-main">
      <!-- A tab's toolbar is a sibling of the scrolling body, not a child, so
           pagination/search/copy stay put while the response scrolls. -->
      <TabsContent class="resp-tab" value="body">
        <div v-if="doc" class="toolbar">
          <template v-if="bodySearchVisible">
            <Input
              ref="bodySearchInputRef"
              v-model="bodyQuery"
              mono
              class="flex-1 min-w-0"
              placeholder="Поиск по ресурсам, полям, значениям…"
              spellcheck="false"
              @keydown.esc="closeBodySearch"
            />
            <span v-if="bodyQuery.trim()" class="search-count">{{ bodyMatchCount }} найдено</span>
            <IconButton variant="outline" hint="Закрыть (Esc)" @click="closeBodySearch"><Icon name="xmark" :size="14" /></IconButton>
          </template>
          <template v-else>
            <template v-if="hasPagination">
              <IconButton variant="outline" :disabled="!pagination.first || !pagination.prev" hint="Первая" @click="follow(pagination.first)"><Icon name="chevrons-left" :size="14" /></IconButton>
              <IconButton variant="outline" :disabled="!pagination.prev" hint="Предыдущая" @click="follow(pagination.prev)"><Icon name="chevron-left" :size="14" /></IconButton>
              <IconButton variant="outline" :disabled="!pagination.next" hint="Следующая" @click="follow(pagination.next)"><Icon name="chevron-right" :size="14" /></IconButton>
              <IconButton variant="outline" :disabled="!pagination.last || !pagination.next" hint="Последняя" @click="follow(pagination.last)"><Icon name="chevrons-right" :size="14" /></IconButton>
            </template>
            <span class="head-spacer"></span>
            <Button v-if="record.source === 'browser'" size="sm" class="open-in-request" @click="openInRequest">Открыть в «Запросе»</Button>
            <Button size="sm" @click="copyBody"><Icon v-if="bodyCopied" name="check" :size="12" /><span>{{ bodyCopied ? 'Скопировано' : 'Копировать' }}</span></Button>
            <Button size="sm" @click="openBodySearch"><span>Поиск</span><kbd class="keycap">{{ searchShortcut }}</kbd></Button>
          </template>
        </div>
        <div v-else-if="record.source === 'browser'" class="toolbar">
          <span class="head-spacer"></span>
          <Button size="sm" class="open-in-request" @click="openInRequest">Открыть в «Запросе»</Button>
        </div>
        <div v-if="doc" class="resp-content">
          <JsonApiTree :doc="doc" :query="bodyQuery" :highlight-key="highlightKey" @select="highlightResource" @inspect="inspectNode" @fetch="follow" />
        </div>
        <!-- Non-JSON:API is read with the Raw tab's own viewer and search — one
             implementation, so the two tabs can't drift apart. -->
        <TextViewerTab
          v-else
          :text="prettyRaw"
          :show-open-in-request="record.source === 'browser'"
          @open-in-request="openInRequest"
        />
      </TabsContent>

      <TabsContent class="resp-tab" value="map">
        <SchemaMap :doc="doc" :highlight-key="highlightKey" @select="openResourceInBody" @fetch="follow" />
      </TabsContent>

      <TabsContent class="resp-tab" value="raw">
        <TextViewerTab :text="prettyRaw" />
      </TabsContent>

      <TabsContent class="resp-tab" value="headers">
        <div class="toolbar">
          <Button size="sm" @click="copyHeaders"><Icon v-if="headersCopied" name="check" :size="12" /><span>{{ headersCopied ? 'Скопировано' : 'Копировать' }}</span></Button>
        </div>
        <div class="resp-content">
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
        </div>
      </TabsContent>

      <TabsContent class="resp-tab" value="cookies">
        <div class="resp-content">
          <CookiesTab :headers="record.responseHeaders" />
        </div>
      </TabsContent>

      <TabsContent class="resp-tab" value="timings">
        <div class="resp-content">
          <TimingsTab :record="record" />
        </div>
      </TabsContent>

      <TabsContent class="resp-tab" value="tests">
        <div class="resp-content">
          <TestsTab v-if="doc" :doc="doc" />
        </div>
      </TabsContent>

      <TabsContent class="resp-tab" value="request">
        <div class="toolbar">
          <span class="request-caption">Показаны после подстановки переменных окружения</span>
        </div>
        <div class="resp-content resp-pad">
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
      </TabsContent>
      <NodeInspector v-if="store.inspector.open" :doc="doc" @close="store.setInspector({ open: false })" @fetch="follow" />
    </div>
    </Tabs>
  </div>
</template>

<style scoped>
@reference "../../style.css";

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

.request-caption {
  @apply mr-auto text-[11.5px] text-text-tertiary;
}

.resp-error {
  @apply py-2.5 px-3 text-red bg-red-soft text-xs;
}

/* One tab's column: only the body scrolls. `flex-1` is what fills the row —
   without it the width collapses to the content. */
.resp-tab {
  @apply flex-1 min-w-0 min-h-0 flex flex-col;
}

.resp-main {
  @apply flex flex-1 min-h-0;
}

.resp-tabs {
  @apply flex flex-col flex-1 min-h-0;
}

.resp-content {
  @apply flex-1 min-h-0 overflow-auto;
}

.resp-pad {
  @apply py-3 px-4 m-0;
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

/* Names .btn to outrank the colour the primitive sets on its own root. */
.btn.open-in-request {
  @apply text-accent;
}
</style>
