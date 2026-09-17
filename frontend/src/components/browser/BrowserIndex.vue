<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Icon from '../ui/Icon.vue'
import { Button } from '../ui/button'
import { Sheet, SheetRow } from '../ui/sheet'
import CaptureFiltersSheet from './CaptureFiltersSheet.vue'
import { useRequestsStore } from '../../stores/requests'
import { useSettings } from '../../composables/useSettings'
import { useToast } from '../../composables/useToast'
import { addressOf } from '../../lib/address'
import { methodInkClass, statusBadgeClass } from '../../lib/format'
import { effectiveTab, tabGroups, tabStats } from '../../lib/recordTabs'
import { formatBytes, formatDate, formatMicros, useMessages } from '../../i18n'
import {
  RecordSource,
  Retention,
  type Record,
} from '../../../bindings/json-inspector/internal/domain'
import {
  BridgeService,
  RecordsService,
  SystemService,
} from '../../../bindings/json-inspector/internal/transport/wails'

// One tab of captured traffic: what it asked for, what came back, and the rules that decided which
// of it arrived. It is a page rather than a viewer because a session is worth reading before its
// requests are: the four numbers say whether anything here is broken, and the table says what.
const store = useRequestsStore()
const { settings } = useSettings()
const toast = useToast()
const { t } = useMessages()

// The tab the pane is about, chosen the way the list chooses it: the same function, so the highlight
// there and the page here cannot point at different tabs.
const tab = computed(() =>
  effectiveTab(tabGroups(store.records.filter((r) => r.source === RecordSource.SourceBrowser)), store.browserTabKey)
)
const items = computed<Record[]>(() => tab.value?.items ?? [])
const stats = computed(() => tabStats(items.value))

// Since when this tab is under capture, as the extension reported it. Zero is "nobody said" — a tab
// restored after a restart that predates the answer — and the line then says less rather than
// something it cannot know.
const since = computed(() => {
  const key = tab.value?.key
  if (!key) return 0
  return store.capture.tabList.find((entry) => String(entry.tabId) === key)?.since ?? 0
})

const meta = computed(() => {
  const count = { n: stats.value.requests }
  const browser = store.capture.browser || t('browser.name')
  if (!store.capture.recording) {
    return t('browser.metaPaused', { browser, ...count })
  }
  if (!since.value) return t('browser.metaCapturingBare', { browser, ...count })
  // The clock and not `formatCheckedAt`: the line reads "since 13:02", and a sentence that spelled
  // out the day would be answering a question nobody asked in a header.
  return t('browser.metaCapturing', {
    browser,
    since: formatDate(since.value, { hour: '2-digit', minute: '2-digit' }),
    ...count,
  })
})

const port = ref(0)

onMounted(async () => {
  port.value = await BridgeService.Port()
})

// The four numbers, in the order the drawing puts them. Each is about this tab and not about the
// session: a page that mixed the two would answer a question nobody asked.
const cards = computed(() => {
  const numbers = stats.value
  const failure = numbers.firstFailure

  return [
    {
      label: t('browser.statRequests'),
      value: String(numbers.requests),
      note: t('browser.statRequestsNote'),
      tone: '',
    },
    {
      label: t('browser.statFailed'),
      value: String(numbers.failed),
      note: failure
        ? t('browser.statFailedNote', {
            status: failure.status || t('browser.failed'),
            path: addressOf(failure.url),
          })
        : t('browser.statFailedNone'),
      tone: numbers.failed ? 'bad' : '',
    },
    {
      label: t('browser.statTransferred'),
      value: formatBytes(numbers.transferred),
      note: t('browser.statTransferredNote'),
      tone: '',
    },
    {
      label: t('browser.statMedian'),
      value: numbers.medianUs ? formatMicros(numbers.medianUs) : t('common.none'),
      note: numbers.fastestUs
        ? t('browser.statMedianNote', { ms: formatMicros(numbers.fastestUs) })
        : t('browser.statMedianNone'),
      tone: '',
    },
  ]
})

const retention = computed(() => {
  switch (settings.value?.historyRetention) {
    case Retention.RetainWeek:
      return t('settings.retentionWeek')
    case Retention.RetainMonth:
      return t('settings.retentionMonth')
    default:
      return t('settings.retentionForever')
  }
})

// ---- the sheets ----------------------------------------------------------

type Sheet = 'extension' | 'filters' | null

const sheet = ref<Sheet>(null)

// What the extension sheet says: four lines about the bridge, one of them coloured because the state
// is the only one the extension itself answers for. The drawing puts a list of tabs here, and the app
// does hold one now — this page states the facts it can state about itself instead.
const facts = computed(() => [
  {
    label: t('browser.extState'),
    tag: store.capture.connected ? t('browser.extConnected') : t('browser.extNotConnected'),
    on: store.capture.connected,
    value: '',
  },
  { label: t('browser.extPort'), tag: '', on: false, value: String(port.value) },
  { label: t('browser.extBrowser'), tag: '', on: false, value: store.capture.browser || t('common.none') },
  { label: t('browser.extTabs'), tag: '', on: false, value: String(store.capture.tabs) },
])

// The rules are the sheet's own: it reads what is stored when it opens and the draft is its business
// until Save. This page only decides that it is the one on screen.
function openFilters() {
  sheet.value = 'filters'
}

async function exportHar() {
  const key = tab.value?.key
  if (!key) return
  const written = await RecordsService.ExportHar(t('files.exportHar'), key)
  if (written) toast.show(t('browser.saved'))
}
</script>

<template>
  <div v-if="tab" class="tab-page">
    <div class="head">
      <span class="head-icon">
        <img v-if="tab.favIconUrl" class="head-favicon" :src="tab.favIconUrl" alt="" />
        <Icon v-else name="globe" :size="23" :stroke-width="1.6" />
      </span>

      <div class="head-text">
        <span class="head-title">{{ tab.title || tab.url || t('history.tab') }}</span>
        <span class="head-meta">{{ meta }}</span>
      </div>

      <button
        type="button"
        class="capture-btn"
        :class="{ stopped: !store.capture.recording }"
        :disabled="!store.capture.connected"
        :title="store.capture.paused ? t('status.resumeCapture') : t('status.pauseCapture')"
        @click="store.toggleCapture()"
      >
        <span class="capture-dot"></span>{{ store.capture.recording ? t('browser.recording') : t('browser.paused') }}
      </button>

      <Button size="page" @click="exportHar">{{ t('browser.exportHar') }}</Button>
    </div>

    <div class="stats">
      <div v-for="card in cards" :key="card.label" class="stat">
        <span class="stat-label">{{ card.label }}</span>
        <span class="stat-value" :class="card.tone">{{ card.value }}</span>
        <span class="stat-note">{{ card.note }}</span>
      </div>
    </div>

    <section class="captured">
      <span class="section-label">{{ t('browser.capturedRequests') }}</span>

      <div class="table">
        <div class="table-head">
          <span>{{ t('browser.colMethod') }}</span>
          <span>{{ t('browser.colPath') }}</span>
          <span>{{ t('browser.colStatus') }}</span>
          <span>{{ t('browser.colTime') }}</span>
          <span>{{ t('browser.colSize') }}</span>
        </div>

        <!-- The rows scroll and the head does not: a tab of two hundred requests is a list to read
             down, and a head that left the screen would stop saying which column is which. -->
        <div class="table-body">
          <button v-for="record in items" :key="record.id" type="button" class="row" @click="store.selectBrowser(record.id)">
            <span class="cell-method mono" :class="methodInkClass(record.method)">{{ record.method }}</span>
            <span class="cell-path mono" :title="record.url">{{ addressOf(record.url) }}</span>
            <span class="cell-status">
              <span class="status" :class="statusBadgeClass(record.status)">{{ record.status }}</span>
            </span>
            <span class="cell-time mono">{{ record.durationUs > 0 ? formatMicros(record.durationUs) : '—' }}</span>
            <span class="cell-size mono">{{ record.responseBytes ? formatBytes(record.responseBytes) : '—' }}</span>
          </button>
        </div>
      </div>
    </section>

    <div class="cards">
      <div class="card">
        <span class="card-title">{{ t('browser.extTitle') }}</span>
        <span class="card-body">{{ store.capture.connected ? t('browser.extBody', { port }) : t('browser.extBodyOff') }}</span>
        <button type="button" class="card-action" @click="sheet = 'extension'">
          {{ t('browser.extAction') }}
        </button>
      </div>

      <div class="card">
        <span class="card-title">{{ t('browser.filtersTitle') }}</span>
        <span class="card-body">{{ t('browser.filtersBody') }}</span>
        <button type="button" class="card-action" @click="openFilters()">
          {{ t('browser.filtersAction') }}
        </button>
      </div>

      <div class="card">
        <span class="card-title">{{ t('browser.retentionTitle') }}</span>
        <span class="card-body">{{ t('browser.retentionBody', { retention }) }}</span>
        <button type="button" class="card-action" @click="SystemService.ShowSettings('requests')">
          {{ t('browser.retentionAction') }}
        </button>
      </div>
    </div>

    <!-- A leaf over the page rather than a tab inside it: the rules are set once and left, and a tab
         that is nearly always closed is a line of the page spent on nothing. -->
    <Sheet
      :open="sheet === 'extension'"
      :title="t('browser.extSheet')"
      :sub="t('browser.extSub')"
      :action="t('browser.extDone')"
      @close="sheet = null"
      @action="sheet = null"
    >
      <div class="rows">
        <SheetRow v-for="fact in facts" :key="fact.label" :label="fact.label">
          <span v-if="fact.tag" class="row-tag" :class="fact.on ? 'on' : 'off'">{{ fact.tag }}</span>
          <span v-else class="row-value mono">{{ fact.value }}</span>
        </SheetRow>
      </div>
    </Sheet>

    <!-- The rules are the settings window's as much as this page's, so the sheet is one component in
         one place: both cards raise the same one, and neither can drift from the other. -->
    <CaptureFiltersSheet v-if="sheet === 'filters'" @close="sheet = null" />
  </div>
</template>

<style scoped>
@reference "../../style.css";

/* The page's own lines, as on the collection page: the handoff is plain HTML and draws every line at
   `normal`, while the window's base is Tailwind's 1.5. */
.tab-page {
  line-height: normal;
  @apply flex-1 min-h-0 overflow-y-auto flex flex-col gap-6.5 bg-bg-panel;
  padding: 28px 32px;
}

.head {
  @apply flex items-start gap-4;
}

.head-icon {
  @apply flex-none inline-flex items-center justify-center w-11 h-11 rounded-xl
         bg-bg-hover text-text-secondary overflow-hidden;
}

.head-favicon {
  @apply w-6 h-6 object-contain;
}

.head-text {
  @apply flex-1 min-w-0 flex flex-col gap-1.5;
}

.head-title {
  @apply text-[22px] font-semibold tracking-[-0.01em] overflow-hidden text-ellipsis whitespace-nowrap;
}

.head-meta {
  @apply text-[13.5px] text-text-secondary;
}

/* The switch that holds capture down. It is the extension's state and not this tab's — the drawing
   puts it here because this is the page about what is being captured. */
/* Not `.cap-btn`: that name is the window's own caption buttons in style.css, and a scoped rule of
   the same name inherits what it does not override — the 46px width the titlebar draws its
   minimise button at, which is what squashed this one. */
.capture-btn {
  @apply flex-none inline-flex items-center gap-[9px] h-9 px-3.5 rounded-[9px] cursor-pointer
         text-[13.5px] font-medium border bg-red-soft border-red-soft;
  font-family: inherit;
  color: var(--red-text);
}

/* The same hover the handoff gives both states of the switch: the fill is not lightened but the whole
   button is, which keeps a red one red and a grey one grey. */
.capture-btn:hover:not(:disabled) {
  filter: brightness(1.04);
}

.capture-btn.stopped {
  @apply bg-bg-inset border-border text-text-secondary;
}

.capture-btn:disabled {
  @apply opacity-50 cursor-default;
}

.capture-dot {
  @apply w-2 h-2 rounded-full;
  background: currentColor;
}

.stats {
  @apply grid grid-cols-4 gap-3;
}

.stat {
  @apply flex flex-col gap-1.5 border border-border rounded-xl;
  padding: 14px 16px;
}

.stat-label {
  @apply text-[11px] font-semibold uppercase tracking-[0.07em] text-text-tertiary;
}

.stat-value {
  @apply text-[20px] font-semibold tracking-[-0.01em];
}

.stat-value.bad {
  color: var(--red-text);
}

.stat-note {
  @apply text-[12.5px] text-text-tertiary overflow-hidden text-ellipsis whitespace-nowrap;
}

.captured {
  @apply flex flex-col gap-2.5;
}

.section-label {
  @apply text-[11px] font-semibold uppercase tracking-[0.07em] text-text-tertiary;
}

/* The rows scroll and the head does not, which is what the collection page does with the same
   problem: a tab of two hundred requests is a list to read down inside a page, and a head that has
   left the screen stops saying which column is which. */
.table {
  @apply flex flex-col border border-border rounded-xl overflow-hidden max-h-[320px];
}

.table-body {
  @apply flex-1 min-h-0 overflow-y-auto;
}

.table-head,
.row {
  @apply grid items-center;
  grid-template-columns: 90px minmax(0, 1fr) 110px 110px 110px;
}

.table-head {
  @apply flex-none bg-bg-inset border-b border-border text-[11px] font-semibold uppercase
         tracking-[0.06em] text-text-tertiary;
}

.table-head > span {
  padding: 10px 16px;
}

.row {
  @apply w-full min-h-[46px] border-0 border-b border-border bg-transparent text-left text-text
         cursor-pointer;
  border-color: color-mix(in srgb, var(--border) 60%, transparent);
  font-family: inherit;
}

.row:hover {
  @apply bg-bg-hover;
}

.row > span {
  padding: 0 16px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cell-method {
  @apply text-[11.5px];
}

.cell-path {
  @apply text-[12.5px];
}

.cell-status {
  @apply flex items-center;
  padding: 0 16px;
}

.status {
  @apply text-[11.5px] font-semibold rounded-md;
  padding: 3px 8px;
}

.cell-time,
.cell-size {
  @apply text-[12.5px] text-text-secondary;
}

.cards {
  @apply grid grid-cols-3 gap-3;
}

.card {
  @apply flex flex-col items-start gap-2 border border-border rounded-xl;
  padding: 16px;
}

.card-title {
  @apply text-[13.5px] font-semibold;
}

.card-body {
  @apply text-[13px] leading-normal text-text-secondary;
}

/* The three cards stand in a row and their sentences differ in length, so the action is pushed to
   the foot of its card: three buttons at three heights read as three different things. */
.card-action {
  @apply self-start mt-auto h-[30px] px-2 -ml-2 border-0 rounded-[7px] bg-transparent cursor-pointer
         text-[13px] font-medium text-accent;
  font-family: inherit;
}

.card-action:hover {
  @apply bg-accent-soft;
}

/* The rows stand in a gap of their own inside the sheet's body. */
.rows {
  @apply flex flex-col gap-0.5;
}

/* The state of the bridge is the one line in these sheets that is a word rather than a number, and
   the pair of fills is what says which word it is. */
.row-tag {
  @apply flex-none text-[12.5px] font-semibold px-[9px] py-1 rounded-md
         bg-bg-hover text-text-tertiary;
}

.row-tag.on {
  @apply bg-green-soft;
  color: var(--green-text);
}

.row-value {
  @apply flex-none text-[12.5px] text-text-secondary;
}

.rows :deep(input) {
  @apply flex-none w-[180px];
}
</style>
