<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Icon from '../ui/Icon.vue'
import { Button, IconButton } from '../ui/button'
import { Input } from '../ui/input'
import { Popover, PopoverTrigger, PopoverContent, PopoverClose } from '../ui/popover'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../ui/tabs'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from '../ui/dropdown-menu'
import type { RecordView } from '../../lib/requestRecord'
import { tryParseJson, prettyJson, highlightJson } from '../../lib/json'
import { formatBytes, formatMicros } from '../../i18n'
import { statusBadgeClass } from '../../lib/format'
import { dataResources, linkHref, isJsonApi, resourceMatchesQuery, type JsonApiDocument } from '../../lib/jsonapi'
import JsonApiTree from '../json/JsonApiTree.vue'
import TextViewerTab from './TextViewerTab.vue'
import SchemaMap from '../json/SchemaMap.vue'
import NodeInspector from '../json/NodeInspector.vue'
import RequestCookiesTab from './RequestCookiesTab.vue'
import TimingsTab from './TimingsTab.vue'
import ScriptsTab from './ScriptsTab.vue'
import { RecordSource } from '../../../bindings/json-inspector/internal/domain'
import { recordSeed, useRequestsStore } from '../../stores/requests'
import { useCollectionsStore } from '../../stores/collections'
import type { InspectorHost } from '../../lib/requestSource'
import { copyToClipboard } from '../../lib/clipboard'
import { CommandService } from '../../../bindings/json-inspector/internal/transport/wails'
import { CommandFormat } from '../../../bindings/json-inspector/internal/usecase/draft'
import { usePlatform } from '../../composables/usePlatform'
import { useToast } from '../../composables/useToast'
import { describeFailure, useMessages } from '../../i18n'
import { focusUrlField } from '../../composables/urlFocus'

// Where this pane is drawn: beside the command line, beside a captured request, or inside a
// collection card. The record is a prop either way — what changes is whose window state it is
// (the inspector) and whether there is anywhere to navigate to.
const props = withDefaults(
  defineProps<{ record: RecordView; source?: 'request' | 'browser' | 'collection' }>(),
  { source: 'request' }
)

const requests = useRequestsStore()
const collections = useCollectionsStore()
const store: InspectorHost = props.source === 'collection' ? collections : requests

// A card shows one response: there is no history behind it to walk and no command line to hand it to.
const hasHistory = computed(() => props.source !== 'collection')
const { t } = useMessages()
const { shortcut } = usePlatform()
const toast = useToast()

type Tab = 'body' | 'map' | 'raw' | 'headers' | 'cookies' | 'timings' | 'scripts' | 'request'
const activeTab = ref<Tab>('body')

// A map, but one rebuilt when the language moves: a plain object would keep the words it was
// created with for as long as the window is open.
const TAB_LABELS = computed<Record<Tab, string>>(() => ({
  body: t('response.tabs.body'),
  map: t('response.tabs.map'),
  raw: t('response.tabs.raw'),
  headers: t('response.tabs.headers'),
  cookies: t('response.tabs.cookies'),
  timings: t('response.tabs.timings'),
  scripts: t('response.tabs.scripts'),
  request: t('response.tabs.request'),
}))

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

// Which page of the collection this answer is, and how many there are. Both halves are read from
// addresses rather than guessed: the record's own address says which page it asked for, and only the
// links say how many the server holds — an answer without `last` gets half a sentence rather than a
// total nobody stated.
function pageNumber(raw: string | undefined): number {
  if (!raw) return 0
  try {
    const value = Number(new URL(raw, 'https://x').searchParams.get('page[number]'))
    return Number.isFinite(value) && value > 0 ? value : 0
  } catch {
    return 0
  }
}

const pageLabel = computed(() => {
  const page = pageNumber(props.record.url)
  if (!page) return ''
  const total = pageNumber(pagination.value.last)
  return total ? t('response.pageOf', { page, total }) : t('response.page', { page })
})

// The version the document declares about itself. The member is typed as unknown because a response
// is whatever the server sent; a version that is not a string is a version this label has no word for.
const jsonapiVersion = computed(() => {
  const declared = (doc.value?.jsonapi ?? null) as { version?: unknown } | null
  return typeof declared?.version === 'string' ? declared.version : ''
})

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

// The active tab is deliberately NOT reset here — switching between history entries (both
// panels share this component) should keep whichever tab you were reading. It only snaps
// back to "Тело" when the new record doesn't have the tab at all, checked against
// `availableTabs` rather than naming tabs here — otherwise a new conditional tab would need
// this list updated too, and it's easy to forget.
watch(
  () => props.record.id,
  () => {
    highlightKey.value = null
    if (!availableTabs.value.includes(activeTab.value)) {
      activeTab.value = 'body'
    }
  }
)

function highlightResource(key: string) {
  highlightKey.value = key
}

// Every click in the tree (even just expanding a node) fires this — it only updates what the
// panel would show, never opens it. Opening is manual: the button or ⌥I, via `toggleInspector`.
function inspectNode(path: string) {
  store.setInspector({ path })
}

function toggleInspector() {
  store.setInspector({ open: !store.inspector.open })
}

function openResourceInBody(key: string) {
  highlightKey.value = key
  activeTab.value = 'body'
}

// Following a link is a plain GET with the headers that worked last time. The record it came from
// is already masked, so the request and the copy history keeps are the same thing here.
function follow(url: string) {
  if (!url || !hasHistory.value) return
  // Switch rails immediately so the user isn't left staring at the stale response.
  requests.activeView = 'request'
  requests.manualId = null
  void requests.sendSpec({ method: 'GET', url, headers: props.record.requestHeaders, body: '' })
}

// The size Go stored, not the size of the text in hand: a body too large to travel with the record
// is reported by its real length, and this label is the one place that would otherwise understate it.
const bodySize = computed(() => props.record.responseBytes ?? new Blob([props.record.responseBody]).size)

function hostPath(url: string): string {
  try {
    const u = new URL(url)
    return u.host + u.pathname
  } catch {
    return url
  }
}

// The verb's plate in the captured request's field: the same object the editor draws, in the two
// colours the drawing gives it — a read is green, and everything else is the accent.
const methodPlate = computed(() =>
  props.record.method === 'GET'
    ? { color: 'var(--green-text)', background: 'var(--green-soft)' }
    : { color: 'var(--accent)', background: 'var(--accent-soft)' }
)

// Values with commas are list-style JSON:API params (include, fields[type]) — chipped one
// item at a time rather than read as one long string.
const urlQueryParams = computed(() => {
  const out: { name: string; values: string[] }[] = []
  try {
    const u = new URL(props.record.url)
    u.searchParams.forEach((value, name) => out.push({ name, values: value.split(',') }))
  } catch {
    // invalid URL — no params to show
  }
  return out
})

// The same query as one line of text, which is what a reader copies.
const queryString = computed(() => {
  try {
    return new URL(props.record.url).search
  } catch {
    return ''
  }
})

// A number reads as a number and a word as a word, which is the whole of what the tint says.
function paramTint(v: string): string {
  return /^-?\d+(\.\d+)?$/.test(v.trim()) ? 'num' : ''
}


// The one place that decides which tabs exist for this record — drives both the tab bar and
// the fallback below, so a tab added here doesn't also need a separate check somewhere else.
const availableTabs = computed<Tab[]>(() => {
  const tabs: Tab[] = ['body']
  if (isJsonApiDoc.value) tabs.push('map')
  tabs.push('raw', 'headers')
  // "Cookies" is the request's own cookie jar, which only a manual send has — a captured
  // response can't show its cookies at all (Set-Cookie is a forbidden header for fetch/XHR,
  // and the browser-side workaround wasn't worth its cost), so the tab would always be empty.
  if (props.record.source === RecordSource.SourceManual) tabs.push('cookies')
  tabs.push('timings')
  // The scripts of a collection run around the requests this app sends, so a report only ever hangs
  // off a record of its own: a captured one has none, and the tab there would always be empty.
  if (props.record.source === RecordSource.SourceManual) tabs.push('scripts')
  // A manual record's request is the one already open in the command line above this viewer —
  // the tab would only repeat it. A captured one has no command line, so there it stays.
  if (props.record.source === RecordSource.SourceBrowser) tabs.push('request')
  return tabs
})

const responseHeaderEntries = computed(() => props.record.responseHeaders)
const requestHeaderEntries = computed(() => props.record.requestHeaders)

// "Назад" walks the record's own source list, not the combined one — going back from a
// captured request must not silently reassign the *manual* selection instead.
const sourceList = computed(() => requests.records.filter((r) => r.source === props.record.source))

const hasPrev = computed(() => {
  if (!hasHistory.value) return false
  const idx = sourceList.value.findIndex((r) => r.id === props.record.id)
  return idx >= 0 && idx < sourceList.value.length - 1
})

function goBack() {
  const idx = sourceList.value.findIndex((r) => r.id === props.record.id)
  if (idx >= 0 && idx < sourceList.value.length - 1) {
    const prev = sourceList.value[idx + 1]
    if (prev.source === RecordSource.SourceBrowser) requests.selectBrowser(prev.id)
    else requests.selectManual(prev.id)
  }
}

function openInRequest() {
  // The whole record becomes the request being composed, the jar it was sent with and all. Which
  // rail is on screen does not change: the button is answered where it was pressed.
  void requests.replace(recordSeed(props.record, props.record.requestBody))
  // Nothing the history shows is what the line is holding any more: this came from a capture, and
  // the row that was lit named a record the line has just stopped being.
  requests.deselectManual()
  requests.setOpenChip(null)
  requests.activeView = 'request'
  focusUrlField()
}

const prettyRaw = computed(() => (isJson.value ? prettyJson(jsonValue.value) : props.record.responseBody))
const searchShortcut = computed(() => shortcut('F'))

const headersCopied = ref(false)

async function copyHeaders() {
  const text = responseHeaderEntries.value.map((h) => `${h.name}: ${h.value}`).join('\n')
  if (await copyToClipboard(text)) {
    headersCopied.value = true
    setTimeout(() => (headersCopied.value = false), 1500)
  }
}

// The link is one of the ways a record is copied, so it says so the way the others do: on the
// button that opened the menu, which is the thing the eye is already on.
async function copyUrl() {
  if (await copyToClipboard(props.record.url)) {
    copied.value = true
    setTimeout(() => (copied.value = false), 1500)
  }
}

const queryCopied = ref(false)

async function copyQuery() {
  if (await copyToClipboard(queryString.value)) {
    queryCopied.value = true
    setTimeout(() => (queryCopied.value = false), 1500)
  }
}

// Search belongs to the tab on screen and to one owner: the button in the row above the tabs and the
// key that names it open the same search, and a tab that has none — a headers table, a timings chart —
// has neither. Three tabs read something long enough to look through, and each looks in its own way:
// the body as resources, Raw as text, Map as types. The body's resource search is this component's
// own and the other two are their viewers', which is why one of the three is answered here.
type TabSearch = { openSearch: () => void; closeSearch: () => void; isSearching: boolean }

const bodyTextViewer = ref<TabSearch | null>(null)
const rawTextViewer = ref<TabSearch | null>(null)
const schemaMap = ref<TabSearch | null>(null)

const treeSearch = computed(() => activeTab.value === 'body' && !!doc.value)

const tabSearch = computed<TabSearch | null>(() => {
  switch (activeTab.value) {
    case 'body':
      return doc.value ? null : bodyTextViewer.value
    case 'raw':
      return rawTextViewer.value
    case 'map':
      return schemaMap.value
    default:
      return null
  }
})

const searchable = computed(() => treeSearch.value || !!tabSearch.value)
const searching = computed(() =>
  treeSearch.value ? bodySearchVisible.value : !!tabSearch.value?.isSearching
)

function openSearch() {
  if (treeSearch.value) openBodySearch()
  else tabSearch.value?.openSearch()
}

function closeSearch() {
  if (treeSearch.value) closeBodySearch()
  else tabSearch.value?.closeSearch()
}

function onWindowKeydown(e: KeyboardEvent) {
  // The physical key rather than the letter: on a Russian layout this key carries «ш».
  if (e.altKey && e.code === 'KeyI') {
    e.preventDefault()
    toggleInspector()
    return
  }
  if ((e.metaKey || e.ctrlKey) && e.code === 'KeyF' && searchable.value) {
    e.preventDefault()
    openSearch()
  } else if (e.key === 'Escape' && searching.value) {
    closeSearch()
  }
}

onMounted(() => window.addEventListener('keydown', onWindowKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onWindowKeydown))

// Wire formats, not words: a cURL command is called cURL in every language. An index signature
// rather than Record<CommandFormat, …>: the enum's `$zero` is not a format, and the map is only
// ever read with one that is.
const COPY_FORMATS: { id: CommandFormat; label: string }[] = [
  { id: CommandFormat.FormatCurl, label: 'cURL' },
  { id: CommandFormat.FormatFetch, label: 'fetch (JS)' },
  { id: CommandFormat.FormatWget, label: 'wget' },
  { id: CommandFormat.FormatHTTPie, label: 'HTTPie' },
  { id: CommandFormat.FormatPowerShell, label: 'PowerShell' },
]

const copied = ref(false)

// The command is written on the side that holds the values: a record carries its secrets as dots
// and nothing else, so what lands in the clipboard is text that can be pasted anywhere.
async function copyAs(format: CommandFormat) {
  let text: string
  try {
    text = await CommandService.Export(props.record.id, format)
  } catch (error) {
    // The record on screen can be gone from the database — the history prunes itself, and another
    // window can clear it — so the copy says what happened instead of doing nothing at all.
    toast.show(t('response.copyFailed', { error: describeFailure(error) }), 'error')
    return
  }
  if (await copyToClipboard(text)) {
    copied.value = true
    setTimeout(() => (copied.value = false), 1500)
  }
}

</script>

<template>
  <div class="resp">
    <!-- The line a captured request gets, and the only one it can: it has no command line to be
         read in, so where it went and what it carried are said here. What the answer was is the
         strip below — a method and a status in this line would be the response said twice. -->
    <div v-if="record.source === 'browser'" class="request-block request-block-solo">
      <div class="bar-row">
        <!-- The field is the editor's own object: the verb standing in its left end, the address after
             it. A captured request has no command line to be read in, so what it was and where it went
             are said here; what came back is the strip below. There is no way back from here — the
             sidebar is where the records are chosen. -->
        <div class="url-field">
          <span class="method-plate" :style="methodPlate">{{ record.method }}</span>
          <span class="req-url mono" :title="record.url">{{ hostPath(record.url) }}</span>
        </div>

        <div v-if="urlQueryParams.length" class="segments">
          <Popover>
            <PopoverTrigger as-child>
              <button class="segment filled">
                <span>{{ t('request.chips.params') }}</span>
                <span class="segment-count">{{ urlQueryParams.length }}</span>
              </button>
            </PopoverTrigger>
            <PopoverContent class="params-menu" align="start" :side-offset="9">
          <div class="params-head">
            <span class="params-title">{{ t('response.paramsTitle') }}</span>
            <span class="params-count mono">{{ urlQueryParams.length }}</span>
            <span class="params-spacer"></span>
            <button class="params-copy" @click="copyQuery">
              {{ queryCopied ? t('common.copied') : t('response.copyQuery') }}
            </button>
            <PopoverClose as-child>
              <IconButton :hint="t('common.close')" size="xl"><Icon name="xmark" :size="14" /></IconButton>
            </PopoverClose>
          </div>

          <!-- One row per parameter rather than one grid with rows in it: the divider between two
               parameters is drawn by the row and spans it, which a cell of a shared grid cannot do. -->
          <div v-for="p in urlQueryParams" :key="p.name" class="params-row">
            <div class="params-name-cell">
              <span class="params-name mono">{{ p.name }}</span>
              <span v-if="p.values.length > 1" class="params-item-count mono">{{ p.values.length }}</span>
            </div>
            <div class="params-chips">
              <span
                v-for="(v, i) in p.values"
                :key="i"
                class="params-chip mono"
                :class="paramTint(v)"
              >{{ v }}</span>
            </div>
          </div>

          <!-- The query as one string, which is what the button above hands over: a chip per value
               says what the request asked for, and this says what the address was. -->
          <div class="params-string">
            <span class="params-string-label">{{ t('response.queryString') }}</span>
            <span class="params-string-text mono">{{ queryString }}</span>
          </div>
          </PopoverContent>
          </Popover>
        </div>
      </div>
    </div>

    <div class="resp-bar">
      <IconButton v-if="hasPrev && record.source !== 'browser'" variant="bare" class="resp-back" :hint="t('response.back')" @click="goBack"><Icon name="chevron-left" :size="14" /></IconButton>
      <!-- The status and the numbers, in the handoff's order: what came back, how long it took, how
           big it was, what it was. The method is not among them — the tab above says whose answer this
           is, and the request's own method is written on the request. -->
      <span class="badge resp-status" :class="statusBadgeClass(record.status)">{{ record.status }}</span>
      <span class="resp-meta">{{ formatMicros(record.durationUs) }}</span>

      <span class="divider"></span>
      <span class="resp-meta">{{ formatBytes(bodySize) }}</span>
      <template v-if="record.contentType">
        <span class="divider"></span>
        <span class="resp-meta resp-type">{{ record.contentType }}</span>
      </template>

      <span class="resp-spacer"></span>

      <Button size="bar" disabled :title="t('response.compareSoon')">{{ t('response.compare') }}</Button>
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <Button size="bar">
            <Icon v-if="copied" name="check" :size="12" />
            <span>{{ copied ? t('common.copied') : t('common.copy') }}</span>
            <svg class="caret" viewBox="0 0 24 24" width="11" height="11" fill="none" aria-hidden="true"><path d="m6 9 6 6 6-6" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"/></svg>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent :side-offset="4">
          <!-- The link comes first and the wire formats after it: one is this request, the others
               are the shapes it can be written in, and the two are read for different reasons. -->
          <DropdownMenuItem @select="copyUrl">{{ t('response.copyUrl') }}</DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem v-for="f in COPY_FORMATS" :key="f.id" @select="copyAs(f.id)">
            {{ f.label }}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Button
        size="bar"
        class="inspector-toggle"
        :class="{ 'inspector-open': store.inspector.open }"
        :title="t('response.inspector', { shortcut: '⌥I' })"
        @click="toggleInspector"
      >{{ t('response.inspectorShort') }}<kbd class="inspector-key">⌥I</kbd></Button>
    </div>

    <div v-if="record.cancelled" class="resp-error">{{ t('response.cancelled') }}</div>
    <div v-else-if="record.error" class="resp-error">{{ t('response.error', { error: record.error }) }}</div>

    <Tabs v-model="activeTab" class="resp-tabs">
      <!-- The row every tab stands in, with the search that reads the tab on screen at its end: a
           button here and not inside each tab's own bar, so the window has one search in one place
           rather than one per tab that happens to have text. -->
      <div class="tabs-row">
        <TabsList>
          <TabsTrigger v-for="tab in availableTabs" :key="tab" :value="tab">{{ TAB_LABELS[tab] }}</TabsTrigger>
        </TabsList>
        <Button v-if="searchable" size="bar" class="tabs-search" @click="openSearch()">
          <span>{{ t('common.search') }}</span><span class="tabs-search-key">{{ searchShortcut }}</span>
        </Button>
      </div>

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
              :placeholder="t('response.searchPlaceholder')"
              spellcheck="false"
              @keydown.esc="closeBodySearch"
            />
            <span v-if="bodyQuery.trim()" class="search-count">{{ t('response.searchFound', { n: bodyMatchCount }) }}</span>
            <IconButton variant="outline" :hint="t('common.close')" @click="closeBodySearch"><Icon name="xmark" :size="14" /></IconButton>
          </template>
          <template v-else>
            <!-- The four steps of the collection as one control, in a groove of its own: they are a
                 single thing the reader moves through, and four separate buttons in a row would say
                 they are four things to press. -->
            <div v-if="hasPagination" class="pager">
              <button type="button" class="pager-btn pager-edge" :disabled="!pagination.first || !pagination.prev" :title="t('response.first')" @click="follow(pagination.first)"><Icon name="chevrons-left" :size="16" :stroke-width="1.9" /></button>
              <button type="button" class="pager-btn" :disabled="!pagination.prev" :title="t('response.previous')" @click="follow(pagination.prev)"><Icon name="chevron-left" :size="16" :stroke-width="1.9" /></button>
              <button type="button" class="pager-btn" :disabled="!pagination.next" :title="t('response.next')" @click="follow(pagination.next)"><Icon name="chevron-right" :size="16" :stroke-width="1.9" /></button>
              <button type="button" class="pager-btn pager-edge" :disabled="!pagination.last || !pagination.next" :title="t('response.last')" @click="follow(pagination.last)"><Icon name="chevrons-right" :size="16" :stroke-width="1.9" /></button>
            </div>
            <span v-if="jsonapiVersion" class="jsonapi-tag mono">jsonapi <span class="jsonapi-ver">v{{ jsonapiVersion }}</span></span>
            <span class="head-spacer"></span>
            <span v-if="pageLabel" class="page-label">{{ pageLabel }}</span>
            <Button v-if="hasHistory && record.source === 'browser'" size="bar" class="open-in-request" @click="openInRequest">{{ t('response.openInRequest') }}</Button>
          </template>
        </div>
        <div v-else-if="record.source === 'browser'" class="toolbar">
          <span class="head-spacer"></span>
          <Button v-if="hasHistory" size="bar" class="open-in-request" @click="openInRequest">{{ t('response.openInRequest') }}</Button>
        </div>
        <div v-if="doc" class="resp-content">
          <JsonApiTree :doc="doc" :query="bodyQuery" :highlight-key="highlightKey" @select="highlightResource" @inspect="inspectNode" @fetch="follow" />
        </div>
        <!-- Non-JSON:API is read with the Raw tab's own viewer and search — one
             implementation, so the two tabs can't drift apart. The ref is what lets the row's one
             search reach this tab's input. -->
        <TextViewerTab
          v-else
          ref="bodyTextViewer"
          :text="prettyRaw"
          :show-open-in-request="record.source === 'browser'"
          @open-in-request="openInRequest"
        />
      </TabsContent>

      <TabsContent class="resp-tab" value="map">
        <SchemaMap ref="schemaMap" :doc="doc" :highlight-key="highlightKey" @select="openResourceInBody" @fetch="follow" />
      </TabsContent>

      <TabsContent class="resp-tab" value="raw">
        <TextViewerTab ref="rawTextViewer" :text="prettyRaw" />
      </TabsContent>

      <TabsContent class="resp-tab" value="headers">
        <div class="toolbar">
          <Button size="sm" @click="copyHeaders"><Icon v-if="headersCopied" name="check" :size="12" /><span>{{ headersCopied ? t('common.copied') : t('common.copy') }}</span></Button>
        </div>
        <div class="resp-content resp-pad resp-white">
          <table class="kv-table">
            <tbody>
              <tr v-for="(h, i) in responseHeaderEntries" :key="i">
                <td class="kv-key mono">{{ h.name }}</td>
                <td class="kv-val mono">{{ h.value }}</td>
              </tr>
              <tr v-if="responseHeaderEntries.length === 0">
                <td class="kv-key" colspan="2">{{ t('response.noResponseHeaders') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </TabsContent>

      <TabsContent class="resp-tab" value="cookies">
        <div class="resp-content">
          <RequestCookiesTab />
        </div>
      </TabsContent>

      <TabsContent class="resp-tab" value="timings">
        <div class="resp-content">
          <TimingsTab :record="record" />
        </div>
      </TabsContent>

      <TabsContent class="resp-tab" value="scripts">
        <div class="resp-content">
          <ScriptsTab :record-id="record.id" />
        </div>
      </TabsContent>

      <TabsContent class="resp-tab" value="request">
        <div class="toolbar">
          <span class="request-caption">{{ t('response.substitutedCaption') }}</span>
        </div>
        <div class="resp-content resp-pad resp-white">
          <div class="kv-row"><span class="kv-label">{{ t('response.method') }}</span><span class="mono">{{ record.method }}</span></div>
          <div class="kv-row"><span class="kv-label">URL</span><span class="mono break">{{ record.url }}</span></div>
          <div class="ja-section-title" style="padding-left: 0">{{ t('response.requestHeaders') }}</div>
          <table class="kv-table">
            <tbody>
              <tr v-for="(h, i) in requestHeaderEntries" :key="i">
                <td class="kv-key mono">{{ h.name }}</td>
                <td class="kv-val mono">{{ h.value }}</td>
              </tr>
              <tr v-if="requestHeaderEntries.length === 0"><td class="kv-key" colspan="2">—</td></tr>
            </tbody>
          </table>
          <div v-if="record.requestBody" class="ja-section-title" style="padding-left: 0">{{ t('response.requestBody') }}</div>
          <pre v-if="record.requestBody" class="code" v-html="highlightJson(record.requestBody)"></pre>
        </div>
      </TabsContent>
      <NodeInspector v-if="store.inspector.open" :doc="doc" :source="source" @close="store.setInspector({ open: false })" @fetch="follow" />
    </div>
    </Tabs>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.resp {
  @apply flex flex-col h-full min-h-0 bg-bg;
}

/* The address in the captured request's field: the field itself, the verb's plate and the sections
   are the shared shapes in style.css — the same objects the editor's bar draws. */
.req-url {
  @apply flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-text;
  font-family: var(--mono);
  font-size: 13px;
}

.resp-bar {
  /* The handoff's own line under this strip: a step quieter than the line under the tab row, so the
     two strips are told apart by the weight of the hairline and not only by what stands in them. */
  @apply flex items-center gap-3.5 h-12 px-5 border-b bg-bg-panel;
  border-color: color-mix(in srgb, var(--border) 60%, transparent);
}

/* The back button is the bar's control in icon form: the same fill, hairline and radius as the
   words beside it, so the row reads as one set of controls at two widths. */
.resp-bar .icon-btn.resp-back {
  width: 30px;
  height: 30px;
  border-radius: 7px;
  background: var(--bg-inset);
  border: 1px solid var(--border);
}

.resp-bar .icon-btn.resp-back:hover {
  @apply bg-bg-hover text-text;
}

.resp-meta {
  @apply text-[13px] whitespace-nowrap text-text-secondary;
  font-family: var(--mono);
}

/* The type is the one value on the bar that is a fact about the answer rather than a measurement of
   it, so it stands a step back from the two numbers. */
.resp-type {
  /* The longest value on the bar and the only one that is allowed to give way: a media type can be
     longer than the room between the numbers and the buttons, and clipped text with an ellipsis is
     still a media type somebody can read the start of. */
  @apply text-text-tertiary overflow-hidden text-ellipsis;
  flex: 0 1 auto;
  min-width: 0;
}

/* The status on the bar: a 20px plate, the height the drawing gives it, with the code alone in it.
   The colour comes from the shared badge classes, as it does in the lists. */
.resp-status {
  height: 20px;
  padding: 0 7px;
  border-radius: 5px;
  font-size: 11px;
  font-weight: 700;
}

.divider {
  @apply w-px h-3.5 bg-border flex-none;
}

.resp-spacer {
  @apply flex-1;
}

/* The row the tabs stand in. The list itself keeps drawing the tabs and their underline; what this
   row adds is the search at the end, which is why the list's own box and padding are taken off and
   given to the row. */
.tabs-row {
  @apply flex items-stretch gap-2 h-[44px] px-4 border-b border-border bg-bg-panel;
}

.tabs-row :deep(.tabs) {
  @apply flex-1 min-w-0 h-auto px-0 border-b-0;
}

.tabs-row .btn.tabs-search {
  @apply self-center gap-2 text-text-secondary;
}

.tabs-search-key {
  @apply text-[12px] text-text-tertiary;
}

/* The four steps as one control: a groove they sit in, and a button per step with no fill of its own
   until the pointer is on it. The two ends are a step quieter than the two the reader moves by —
   they jump to the ends of the collection, and the steps beside them are what a page is turned with. */
.pager {
  @apply flex gap-0.5 flex-none rounded-lg p-0.5 bg-bg-hover;
}

.pager-btn {
  @apply inline-flex items-center justify-center w-8 h-7 rounded-md border-0 bg-transparent
         cursor-pointer text-text-secondary;
  padding: 0;
  --wails-draggable: no-drag;
}

.pager-btn.pager-edge {
  @apply text-text-tertiary;
}

.pager-btn:hover:not(:disabled) {
  @apply bg-bg-active text-text;
}

.pager-btn:disabled {
  @apply opacity-50 cursor-default;
}

.pager-btn:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
}

/* What the answer says about itself: the version of the format it is written in, read from the
   document rather than from its content type. */
.jsonapi-tag {
  @apply flex-none ml-2 text-[13px] text-text-tertiary;
}

.jsonapi-ver {
  @apply text-text-secondary;
}

.page-label {
  @apply flex-none text-[13px] text-text-tertiary;
}

.request-caption {
  @apply mr-auto text-xs text-text-tertiary;
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

/* The padding the design gives a tab that is a table: 18px above and below what it holds and 20 at
   the sides, so a card on this surface stands the same distance off every edge. */
.resp-pad {
  @apply py-[18px] px-5 m-0;
}

/* The handoff's "table" tabs (Raw, Headers, Cookies, Timings, Tests, Request) sit on a solid
   panel; Body/Map keep the gray canvas underneath their own white cards, so this is opt-in
   per tab rather than a default on `.resp-content`. */
.resp-white {
  @apply bg-bg-panel;
}

/* A tab's table is a card of its own on that surface: the design draws it with its own border and a
   12px radius, which is why this one is neither a bare table nor full-bleed. */
.kv-table {
  @apply w-full rounded-xl overflow-hidden border border-border;
  border-collapse: separate;
  border-spacing: 0;
  font-family: var(--mono);
  font-size: 13px;
}

/* 44px rows with the name and the value centred in them, and no line under the last: the card's own
   border is the line the table ends on. The height is the row's rather than the cells', so that a value
   long enough to wrap leaves the two cells — and the hover fill across them — the same height. */
.kv-table tr {
  height: 44px;
}

.kv-key {
  @apply text-text-secondary whitespace-nowrap border-b border-border;
  width: 300px;
  padding: 0 16px;
}

.kv-val {
  @apply text-text break-all select-text border-b border-border;
  padding: 0 16px;
}

.kv-table tr:last-child .kv-key,
.kv-table tr:last-child .kv-val {
  border-bottom: 0;
}

.kv-table tr:hover .kv-key,
.kv-table tr:hover .kv-val {
  background: var(--bg-hover);
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


.params-head {
  @apply flex items-center gap-2.5 pb-2.5;
}

.params-title {
  @apply text-sm font-semibold;
}

.params-count {
  @apply text-[13px] text-text-tertiary;
  font-family: var(--mono);
}

.params-spacer {
  @apply flex-1;
}

.params-copy {
  @apply flex-none h-7 px-2.5 rounded-[7px] border cursor-pointer text-[13px] text-text
         bg-bg-inset border-border-strong;
  font-family: inherit;
  --wails-draggable: no-drag;
}

.params-copy:hover {
  @apply bg-bg-hover;
}

.params-row {
  @apply grid items-start gap-3 py-2.5 px-1.5;
  grid-template-columns: 170px minmax(0, 1fr);
  border-top: 1px solid color-mix(in srgb, var(--border) 60%, transparent);
}

.params-name-cell {
  @apply flex items-baseline gap-[7px];
}

.params-name {
  @apply text-[13px] text-text;
}

.params-item-count {
  @apply text-[12px] text-text-tertiary;
}

.params-chips {
  @apply flex flex-wrap gap-[5px];
}

.params-chip {
  @apply text-[13px] text-accent py-[3px] px-2 rounded-md;
  background: var(--accent-soft);
}

/* A number is not a word, and the pair of colours is what says which of the two a value is. */
.params-chip.num {
  @apply text-tok-num bg-bg-hover;
}

/* The query as one string: what the chips say value by value, in the shape it is pasted in. */
.params-string {
  @apply flex gap-3 mt-2.5 p-3 rounded-[8px] bg-bg-inset;
}

.params-string-label {
  @apply flex-none pt-[2px] text-[11px] font-semibold uppercase tracking-[0.07em] text-text-tertiary;
}

.params-string-text {
  @apply flex-1 min-w-0 text-[13px] break-all;
  line-height: 1.6;
  color: var(--text-secondary);
}

/* Names .btn to outrank the colour the primitive sets on its own root. */
.btn.open-in-request {
  @apply text-accent;
}

/* The pane's toggle is the bar's only button that answers for a state: an open pane fills it with
   the accent, which is what the handoff draws. `.btn` is written twice there because the primitive's
   own hover paints the background as well, and an open pane must not grey out under the pointer —
   the drawing lightens it instead. */
.resp-bar .btn.btn.inspector-toggle {
  gap: 8px;
}

.resp-bar .btn.btn.inspector-open,
.resp-bar .btn.btn.inspector-open:hover {
  @apply text-accent bg-accent-soft border-accent-soft;
}

.resp-bar .btn.btn.inspector-open:hover {
  filter: brightness(1.03);
}

/* The key beside the word, at the handoff's own size for this one: fainter than the label and not in
   the small caps a keycap draws, because the handoff writes it plain. */
.inspector-key {
  @apply text-[12px] opacity-70;
  font-family: inherit;
}

/* The chevron of a menu button says the label has more behind it, so it stays behind the label —
   a step fainter than the word it points away from. */
.caret {
  @apply flex-none text-text-tertiary;
}
</style>
