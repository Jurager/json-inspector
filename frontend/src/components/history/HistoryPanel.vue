<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Icon from '../ui/Icon.vue'
import { Button, IconButton } from '../ui/button'
import PanelFilter from '../ui/PanelFilter.vue'
import { useListKeys } from '../../composables/useListKeys'
import { useRequestsStore } from '../../stores/requests'
import { RecordSource, type Record } from '../../../bindings/json-inspector/internal/domain'
import { statusBadgeClass } from '../../lib/format'
import { addressOf } from '../../lib/address'
import { formatDate, formatMicros, useMessages } from '../../i18n'

const props = defineProps<{ sourceKind: RecordSource }>()

const store = useRequestsStore()

const { t } = useMessages()

const browser = computed(() => props.sourceKind === RecordSource.SourceBrowser)

const emptyHint = computed(() =>
  browser.value
    ? t('history.emptyCaptured')
    : t('history.emptySent')
)

const records = computed(() => store.records.filter((r) => r.source === props.sourceKind))
const activeId = computed(() =>
  props.sourceKind === RecordSource.SourceBrowser ? store.browserId : store.manualId
)

function select(id: string) {
  if (props.sourceKind === RecordSource.SourceBrowser) store.selectBrowser(id)
  else store.selectManual(id)
}

function clearAll() {
  void store.clearRecords(records.value.map((r) => r.id))
}

function timeLabel(startedAt: number): string {
  const d = new Date(startedAt)
  return formatDate(d, { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

const query = ref('')

function matches(r: Record, q: string): boolean {
  return (
    r.method.toLowerCase().includes(q) ||
    String(r.status).includes(q) ||
    r.url.toLowerCase().includes(q)
  )
}

const filteredRecords = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return records.value
  return records.value.filter((r) => matches(r, q))
})

function dateLabel(startedAt: number): string {
  const d = new Date(startedAt)
  const start = new Date(d)
  start.setHours(0, 0, 0, 0)
  const now = new Date()
  const today = new Date(now)
  today.setHours(0, 0, 0, 0)
  const yesterday = new Date(today)
  yesterday.setDate(yesterday.getDate() - 1)
  const dayMs = start.getTime()
  if (dayMs === today.getTime()) return t('history.today')
  if (dayMs === yesterday.getTime()) return t('history.yesterday')
  return formatDate(d, { day: 'numeric', month: 'long' })
}

const manualGroups = computed(() => {
  const out: { label: string; items: Record[] }[] = []
  for (const r of filteredRecords.value) {
    const label = dateLabel(r.startedAt)
    const last = out[out.length - 1]
    if (last && last.label === label) last.items.push(r)
    else out.push({ label, items: [r] })
  }
  return out
})

interface TabGroup {
  key: string
  title: string
  url: string
  favIconUrl: string
  items: Record[]
}

const tabGroups = computed<TabGroup[]>(() => {
  const map = new Map<string, TabGroup>()
  for (const r of records.value) {
    const key = r.tabId != null ? String(r.tabId) : r.tabURL || 'unknown'
    let g = map.get(key)
    if (!g) {
      g = { key, title: r.tabTitle || '', url: r.tabURL || '', favIconUrl: r.favIconUrl || '', items: [] }
      map.set(key, g)
    }
    g.items.push(r)
    if (!g.title && r.tabTitle) g.title = r.tabTitle
    if (!g.favIconUrl && r.favIconUrl) g.favIconUrl = r.favIconUrl
    if (!g.url && r.tabURL) g.url = r.tabURL
  }
  return Array.from(map.values()).sort((a, b) => b.items[0].startedAt - a.items[0].startedAt)
})

const filteredGroups = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return tabGroups.value
  return tabGroups.value
    .map((g) => ({ ...g, items: g.items.filter((r) => matches(r, q)) }))
    .filter((g) => g.items.length > 0)
})

const isEmptyFiltered = computed(() =>
  browser.value ? filteredGroups.value.length === 0 : filteredRecords.value.length === 0
)

function isRecording(g: TabGroup): boolean {
  return store.capture.recording && filteredGroups.value[0]?.key === g.key
}

const collapsed = ref<Set<string>>(new Set())
const brokenFavicons = ref<Set<string>>(new Set())

// The rows the arrows walk: what is drawn, in the order it is drawn. A collapsed tab holds its
// requests back, and a filtered-out one is not on screen to be walked to.
const rowIds = computed(() => {
  if (browser.value) {
    return filteredGroups.value
      .filter((g) => !collapsed.value.has(g.key))
      .flatMap((g) => g.items.map((r) => r.id))
  }
  return manualGroups.value.flatMap((g) => g.items.map((r) => r.id))
})

useListKeys({
  ids: () => rowIds.value,
  current: () => activeId.value,
  move: (id) => void select(id),
  selected: '.history-panel .item.active',
})

function toggleGroup(key: string) {
  const next = new Set(collapsed.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  collapsed.value = next
}

function markBroken(key: string) {
  const next = new Set(brokenFavicons.value)
  next.add(key)
  brokenFavicons.value = next
}

function clearGroup(g: TabGroup) {
  void store.clearRecords(g.items.map((r) => r.id))
}

function hostnameOf(url: string): string {
  try {
    return new URL(url).hostname
  } catch {
    return ''
  }
}

function groupLabel(g: TabGroup): string {
  return g.title || hostnameOf(g.url) || t('history.tab')
}

function groupHue(key: string): number {
  let h = 0
  for (let i = 0; i < key.length; i++) h = (h * 31 + key.charCodeAt(i)) >>> 0
  return h % 360
}

// A deep link's tab may have nothing yet, but the link is clicked right after the page loads
// and the first request lands a moment later — so the request is kept.
function focusDeepLinkedTab() {
  if (props.sourceKind !== RecordSource.SourceBrowser) return
  const tabId = store.focusTabId
  if (tabId == null) return
  const g = tabGroups.value.find((x) => x.key === String(tabId))
  if (!g || g.items.length === 0) return
  const next = new Set(collapsed.value)
  next.delete(g.key)
  collapsed.value = next
  // Items are newest-first, so the first one is the request just made.
  store.selectBrowser(g.items[0].id)
  store.focusTabId = null
}

watch(() => [store.focusTabId, store.records.length, props.sourceKind] as const, focusDeepLinkedTab, {
  immediate: true,
})
</script>

<template>
  <div class="history-panel">
    <div class="panel-head">
      <span class="panel-title">{{ browser ? t('history.captured') : t('history.history') }}</span>
      <Button variant="ghost" class="panel-clear" :disabled="records.length === 0" @click="clearAll">{{ t('history.clear') }}</Button>
    </div>

    <div v-if="records.length === 0 && !browser" class="empty">
      <Icon name="clock" :size="30" :stroke-width="1.6" class="empty-icon" />
      <div class="empty-text">
        <span class="empty-hint">{{ emptyHint }}</span>
      </div>
    </div>

    <div v-else-if="records.length > 0 && isEmptyFiltered" class="no-results">{{ t('common.nothingFound') }}</div>

    <ul v-else-if="!browser" class="list">
      <template v-for="g in manualGroups" :key="g.label">
        <li class="date-sep">{{ g.label }}</li>
        <li
          v-for="r in g.items"
          :key="r.id"
          class="item"
          :class="{ active: r.id === activeId }"
          role="button"
          tabindex="0"
          @click="select(r.id)"
          @keydown.enter="select(r.id)"
          @keydown.space.prevent="select(r.id)"
        >
          <span class="item-head">
            <span class="item-method">{{ r.method }}</span>
            <span class="item-status" :class="statusBadgeClass(r.status)">{{ r.status }}</span>
            <span class="item-time">{{ timeLabel(r.startedAt) }}</span>
          </span>
          <span class="item-path mono" :title="r.url">{{ addressOf(r.url) }}</span>
        </li>
      </template>
    </ul>

    <div v-else class="list list-browser">
      <section v-for="g in filteredGroups" :key="g.key" class="group">
        <div
          class="group-head"
          role="button"
          tabindex="0"
          @click="toggleGroup(g.key)"
          @keydown.enter="toggleGroup(g.key)"
          @keydown.space.prevent="toggleGroup(g.key)"
        >
          <span class="caret" :class="{ open: !collapsed.has(g.key) }">
            <Icon name="chevron-right" :size="13" />
          </span>
          <img
            v-if="g.favIconUrl && !brokenFavicons.has(g.key)"
            class="favicon"
            :src="g.favIconUrl"
            alt=""
            @error="markBroken(g.key)"
          />
          <span v-else class="avatar" :style="{ background: `hsl(${groupHue(g.key)}, 58%, 45%)` }">
            {{ groupLabel(g).charAt(0).toUpperCase() }}
          </span>
          <span class="group-title">{{ groupLabel(g) }}</span>
          <span v-if="isRecording(g)" class="recording-label">
            <span class="recording-dot"></span>{{ t('history.rec') }}
          </span>
          <span class="group-count">{{ g.items.length }}</span>
          <IconButton variant="danger" size="sm" :hint="t('history.clearTab')" @click.stop="clearGroup(g)"><Icon name="trash" :size="13" /></IconButton>
        </div>

        <ul v-show="!collapsed.has(g.key)" class="group-items">
          <li
            v-for="r in g.items"
            :key="r.id"
            class="item"
            :class="{ active: r.id === activeId }"
            role="button"
            tabindex="0"
            @click="select(r.id)"
            @keydown.enter="select(r.id)"
            @keydown.space.prevent="select(r.id)"
          >
            <span class="item-method mono">{{ r.method }}</span>
            <span class="item-status" :class="statusBadgeClass(r.status)">{{ r.status }}</span>
            <span class="item-path mono" :title="r.url">{{ addressOf(r.url) }}</span>
            <span v-if="r.durationUs > 0" class="item-duration">{{ formatMicros(r.durationUs) }}</span>
          </li>
        </ul>
      </section>
    </div>

    <PanelFilter
      v-if="records.length > 0"
      v-model="query"
      :placeholder="t('history.filter')"
    />
  </div>
</template>

<style scoped>
@reference "../../style.css";

.history-panel {
  /* The seam against the content belongs to the panel's frame, which knows which edge it is on. */
  @apply relative flex flex-col h-full min-h-0 bg-bg-panel;
}

/* The header carries no hairline any more: the panel is one surface with its list, and the lines the
   window used to run across it are gone (the design's «меньше линий» pass). */
.panel-head {
  @apply flex items-center justify-between h-[52px] px-4;
}

.panel-title {
  @apply text-[14px] font-semibold text-text;
}

/* Written against the button's own classes, not beside them: the shared ghost button carries its own
   padding and size, and a single class of the panel's loses to it. The border goes with the fill, and
   the line box is set rather than left to the window's 1.5 — this is a small button, not a paragraph. */
.panel-head .btn.panel-clear {
  @apply text-[13px] font-normal leading-[17px] border-0 py-[5px] px-2;
}

/* The separator is the panel's own line, so it breaks out of the list's inset rather than sitting
   inside it: the label starts where the header's title does. */
.date-sep {
  @apply -mx-2.5 text-[11px] font-semibold uppercase leading-[15px] tracking-[0.07em] text-text-tertiary pt-1 px-4 pb-2;
}

.no-results {
  @apply pt-3 px-4 pb-3 text-center text-text-tertiary text-[13px];
}

/* The block itself is the window's empty state (style.css); a panel only adds the padding and the
   centred text its own width asks for. */
.empty {
  @apply p-4 text-center;
}

.empty-hint {
  @apply max-w-[220px];
}

/* Both lists are a column of rows with a gap, and the two rails differ only in that gap: the sent rows
   are their own paragraph each and stand 4px apart, while the captured ones read as one stack of 2px. */
.list {
  @apply flex-1 min-h-0 overflow-auto flex flex-col gap-1 px-2.5;
}

.list-browser {
  @apply gap-0.5;
}

.group {
  @apply flex flex-col gap-0.5;
}

.group-head {
  @apply flex items-center gap-[9px] w-full h-[38px] px-2.5 border-none rounded-[9px] bg-transparent text-text text-[13px] text-left cursor-pointer select-none;
  --wails-draggable: no-drag;
}

.group-head:hover {
  @apply bg-bg-hover;
}

.caret {
  @apply inline-flex items-center justify-center flex-none w-[13px] h-[13px] text-text-secondary;
  transition: transform 0.12s ease;
}

.caret.open {
  @apply rotate-90;
}

.favicon {
  @apply flex-none w-5 h-5 rounded-md object-contain;
  border: 1px solid var(--border);
  background: var(--card);
}

.avatar {
  @apply flex-none w-5 h-5 rounded-md text-white text-[10.5px] font-semibold inline-flex items-center justify-center leading-none uppercase;
}

.group-title {
  @apply flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap;
}

.recording-label {
  @apply flex-none inline-flex items-center gap-[5px] text-[11px] text-red;
}

.recording-dot {
  @apply w-[7px] h-[7px] rounded-full flex-none;
  background: var(--green);
}

.group-count {
  @apply flex-none text-[11.5px] leading-[15px] text-text-tertiary;
  font-variant-numeric: tabular-nums;
}

.group-head :deep(.icon-btn) {
  opacity: 0.55;
}

.group-head:hover :deep(.icon-btn) {
  opacity: 1;
}

.group-items {
  @apply list-none m-0 p-0 flex flex-col gap-0.5;
}

/* Two lines: what the attempt was — method, status, when — and under it the address it went to. The
   row that is open is filled with the accent rather than tinted by it, so the one being looked at is
   the one the eye lands on. */
.item {
  @apply flex flex-col gap-1.5 py-[11px] px-3 rounded-[9px] cursor-pointer;
}

.item-head {
  @apply flex items-center gap-2;
}

/* A captured row is one line instead — method, status, address, how long — and it is the height of the
   tab above it: a tab of thirty requests has to stay a column the eye can run down. The address takes
   the slack, so a row without a duration (the extension could not time it) lays out the same. */
.group-items .item {
  @apply flex-row items-center gap-[9px] h-[38px] py-0 pl-[30px] pr-2.5;
}

.group-items .item-method {
  @apply tracking-normal;
}

.group-items .item-path {
  @apply flex-1 text-[12.5px] leading-[15px];
}

.group-items .item-duration {
  @apply flex-none text-[11.5px] leading-[15px] text-text-tertiary;
  font-variant-numeric: tabular-nums;
}

.group-items .item.active .item-duration {
  color: rgba(255, 255, 255, 0.75);
}

.item:hover {
  @apply bg-bg-hover;
}

.item.active {
  background: var(--accent);
}

.item.active .item-path {
  color: var(--accent-text);
}

.item.active .item-time {
  color: rgba(255, 255, 255, 0.75);
}

.item.active .item-method,
.item.active .item-status {
  background: rgba(255, 255, 255, 0.22);
  color: #fff;
}

/* The method and the status are told apart by their fill alone, and neither shifts the row. The line
   box is set rather than left to the window's own 1.5, which is a paragraph's leading and not a badge's:
   without it the badge is 22 tall inside a 20-tall header. */
.item-method {
  @apply flex-none inline-flex items-center px-1.5 py-[3px] rounded-[5px] text-[10.5px] leading-[14px] font-medium tracking-[0.04em];
  font-variant-numeric: tabular-nums;
  background: var(--bg-hover);
  color: var(--text-secondary);
}

.item-status {
  @apply flex-none inline-flex items-center px-1.5 py-[3px] rounded-[5px] text-[10.5px] leading-[14px] font-medium;
  font-variant-numeric: tabular-nums;
}

.item-path {
  @apply min-w-0 text-[13px] leading-[15px] text-text overflow-hidden text-ellipsis whitespace-nowrap;
  font-family: var(--mono);
}

.item-time {
  @apply ml-auto flex-none text-[11.5px] leading-[15px] text-text-tertiary;
  font-variant-numeric: tabular-nums;
}
</style>
