<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Icon from '../ui/Icon.vue'
import { IconButton } from '../ui/button'
import PanelFilter from '../ui/PanelFilter.vue'
import { useListKeys } from '../../composables/useListKeys'
import { useRequestsStore } from '../../stores/requests'
import { RecordSource, type Record } from '../../../bindings/json-inspector/internal/domain'
import { splitAddress } from '../../lib/address'
import { effectiveTab, groupHue, groupLabel, tabGroups, type TabGroup } from '../../lib/recordTabs'
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

// What the bin at the top of the panel is about: the two panels are the two halves of one sidebar,
// and "clear" on its own would not say which half it empties.
const clearHint = computed(() => (browser.value ? t('history.clearCaptured') : t('history.clear')))

function clearAll() {
  void store.clearRecords(records.value.map((r) => r.id))
}

// What the row no longer says out loud. The list keeps three marks — the method, the address and the
// outcome — and the code and the time it took move into the tooltip, where whoever needs the number
// finds it without every row carrying it.
function rowTitle(r: Record): string {
  const parts = [`${r.method} ${r.status}`]
  if (r.durationUs > 0) parts.push(formatMicros(r.durationUs))
  return parts.join(' · ')
}

// The strip at the end of a row, in the colour of the outcome. A captured redirect is a request that
// went where it was meant to and counts as one; a request that never got an answer is red.
function statusTone(status: number, redirectIsFine = false): string {
  const fine = status >= 200 && status < (redirectIsFine ? 400 : 300)
  return fine ? 'is-ok' : 'is-bad'
}

const hostOf = (url: string): string => splitAddress(url).host
const pathOf = (url: string): string => splitAddress(url).path

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

const allGroups = computed<TabGroup[]>(() => tabGroups(records.value))

const filteredGroups = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return allGroups.value
  return allGroups.value
    .map((g) => ({ ...g, items: g.items.filter((r) => matches(r, q)) }))
    .filter((g) => g.items.length > 0)
})

const isEmptyFiltered = computed(() =>
  browser.value ? filteredGroups.value.length === 0 : filteredRecords.value.length === 0
)

// Whether this tab is the one being captured: the extension says which tabs it has armed and when,
// so the label is that answer and not a guess about which group received the last request.
function isRecording(g: TabGroup): boolean {
  if (!store.capture.recording) return false
  return store.capture.tabList.some((tab) => String(tab.tabId) === g.key)
}

// Since when this tab is under capture, or nothing when the extension did not say — a tab restored
// after a restart that predates the answer.
function sinceOf(g: TabGroup): number {
  return store.capture.tabList.find((tab) => String(tab.tabId) === g.key)?.since ?? 0
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
  selected: '.history-panel .panel-row.active',
})

// The tab the pane is about. It is the effective one and not the chosen one, so the highlight and
// the page can never point at different tabs: with nothing chosen — or with a tab whose records are
// gone — the pane shows the newest, and this says the newest too.
const activeTabKey = computed(() => {
  if (store.browserId) return null
  if (props.sourceKind !== RecordSource.SourceBrowser) return null
  return effectiveTab(allGroups.value, store.browserTabKey)?.key ?? null
})

// The head opens the tab: the page for it is what a reader wants from a row that names a page. The
// caret is the gesture of the list — opening and closing it — and stays where it was.
function openTab(key: string) {
  store.selectBrowserTab(key)
}

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

// A deep link names a tab, and the store has already made it the one on screen. What is left for
// this panel is to open the row: a tab whose requests are hidden describes nothing.
watch(
  () => [store.browserTabKey, allGroups.value.length, props.sourceKind] as const,
  ([key]) => {
    if (props.sourceKind !== RecordSource.SourceBrowser || !key) return
    if (!allGroups.value.some((g) => g.key === key)) return
    const next = new Set(collapsed.value)
    next.delete(key)
    collapsed.value = next
  },
  { immediate: true }
)
</script>

<template>
  <div class="history-panel">
    <div class="panel-head">
      <span class="panel-title">{{ browser ? t('history.captured') : t('history.history') }}</span>
      <!-- The bin and not the word: the drawing puts an icon at the end of this bar, and the word it
           used to carry took a line of the widest row in the panel. What it clears is in its title —
           both panels are one sidebar, and each bin has to name its own pile. -->
      <button
        type="button"
        class="panel-icon danger"
        :title="clearHint"
        :disabled="records.length === 0"
        @click="clearAll"
      >
        <Icon name="trash" :size="15" :stroke-width="1.8" />
      </button>
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
        <li class="panel-group">{{ g.label }}</li>
        <li
          v-for="r in g.items"
          :key="r.id"
          class="panel-row"
          :class="{ active: r.id === activeId }"
          role="button"
          tabindex="0"
          @click="select(r.id)"
          @keydown.enter="select(r.id)"
          @keydown.space.prevent="select(r.id)"
        >
          <span class="panel-method">{{ r.method }}</span>
          <span class="panel-address" :title="rowTitle(r)">
            <span class="panel-host">{{ hostOf(r.url) }}</span>{{ pathOf(r.url) }}
          </span>
          <span class="panel-strip" :class="statusTone(r.status)"></span>
        </li>
      </template>
    </ul>

    <div v-else class="list list-browser">
      <section v-for="g in filteredGroups" :key="g.key" class="group">
        <div
          class="panel-row row-site"
          :class="{ active: g.key === activeTabKey }"
          role="button"
          tabindex="0"
          @click="openTab(g.key)"
          @keydown.enter="openTab(g.key)"
          @keydown.space.prevent="openTab(g.key)"
        >
          <span
            class="caret"
            :class="{ open: !collapsed.has(g.key) }"
            @click.stop="toggleGroup(g.key)"
          >
            <Icon name="chevron-right" :size="11" />
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
          <span class="row-title">{{ groupLabel(g) }}</span>
          <!-- The marker is the dot and nothing else: the word that used to stand beside it took the
               width a site's own name needs, and the colour says the same thing faster. -->
          <span v-if="isRecording(g)" class="recording-dot"></span>
          <span class="panel-count">{{ g.items.length }}</span>
          <IconButton variant="danger" size="sm" :hint="t('history.clearTab')" @click.stop="clearGroup(g)"><Icon name="trash" :size="14" :stroke-width="1.8" /></IconButton>
        </div>

        <ul v-show="!collapsed.has(g.key)" class="group-items">
          <li
            v-for="r in g.items"
            :key="r.id"
            class="panel-row"
            :class="{ active: r.id === activeId }"
            role="button"
            tabindex="0"
            @click="select(r.id)"
            @keydown.enter="select(r.id)"
            @keydown.space.prevent="select(r.id)"
          >
            <span class="panel-method">{{ r.method }}</span>
            <span class="panel-address" :title="rowTitle(r)">
              <span class="panel-host">{{ hostOf(r.url) }}</span>{{ pathOf(r.url) }}
            </span>
            <span class="panel-strip" :class="statusTone(r.status, true)"></span>
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
  /* The seam against the content belongs to the panel's frame, which knows which edge it is on, and
     the fill belongs to that frame too: the panel wears the sidebar's glass and draws nothing under
     it, so the blur has the tint behind it to work on. The head, the row and the foot are the shared
     class in style.css. */
  @apply relative flex flex-col h-full min-h-0;
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

/* Both lists are a stack of rows with no gap and no inset of their own: the row carries its own
   padding, and a second rhythm between the rows would fight the one the design set. */
.list {
  @apply flex-1 min-h-0 overflow-auto flex flex-col;
}

.group {
  @apply flex flex-col;
}

.group-items {
  @apply list-none m-0 p-0 flex flex-col;
}

/* A captured request stands under the site it belongs to and is indented under it, so the two read
   as a heading and its list rather than as one flat column. */
.group-items .panel-row {
  padding-left: 28px;
}

/* The site a group belongs to is that group's heading: its own name in the weight a heading has. */
.row-site {
  padding-right: 6px;
}

.row-site .row-title {
  @apply flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap font-semibold;
}

.caret {
  @apply inline-flex items-center justify-center flex-none w-[11px] h-[11px] text-text-tertiary;
  transition: transform 0.12s ease;
}

.caret.open {
  @apply rotate-90;
}

.favicon {
  @apply flex-none w-4 h-4 rounded-sm object-contain;
  border: 1px solid var(--border);
  background: var(--bg-panel);
}

.avatar {
  @apply flex-none w-4 h-4 rounded-sm text-white text-[10px] font-semibold inline-flex items-center justify-center leading-none uppercase;
  /* The line box keeps room for the descenders a capital never reaches, so a single letter sits
     about half of that below the middle of the square. Padding at the foot shrinks the box the line
     is centred in, which lifts it by half the padding — and in em, so every size is the same. */
  padding-bottom: 0.09em;
}

/* The dot is the whole marker: the word that used to stand beside it took the width the site's own
   name needs, and the colour says the same thing faster. */
.recording-dot {
  @apply flex-none w-[6px] h-[6px] rounded-full;
  background: var(--red);
}

.group .panel-row :deep(.icon-btn) {
  opacity: 0.55;
}

.group .panel-row:hover :deep(.icon-btn) {
  opacity: 1;
}
</style>
