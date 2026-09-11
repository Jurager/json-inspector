<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from './Icon.vue'
import { useRequestsStore } from '../stores/requests'
import type { RequestRecord } from '../lib/types'
import { statusClass } from '../lib/json'

// One panel, two sources: the manually-built requests ("Запрос" tab) and the
// ones captured by the browser extension ("Браузер" tab). They're the same
// list of request/response records, just filtered differently and — for the
// browser source only — grouped by the tab they came from. Sharing one
// component keeps their wording and styling identical instead of two panels
// silently drifting apart.
const props = defineProps<{ source: 'manual' | 'browser' }>()

const store = useRequestsStore()

const emptyHint = computed(() =>
  props.source === 'browser'
    ? 'Установите и активируйте расширение. Перехваченные запросы появятся здесь.'
    : 'Здесь появятся запросы, отправленные вручную.'
)

const records = computed(() => store.requests.filter((r) => r.source === props.source))
const activeId = computed(() => (props.source === 'browser' ? store.browserId : store.manualId))

function select(id: string) {
  if (props.source === 'browser') store.selectBrowser(id)
  else store.selectManual(id)
}

function clearAll() {
  store.clearRequests(records.value.map((r) => r.id))
}

function timeLabel(startedAt: number): string {
  const d = new Date(startedAt)
  return d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

// Path + query only — the scheme and host are redundant here (the host already
// shows in the browser group header or the command line), and the full URL is
// kept in the row's title tooltip.
function pathOf(url: string): string {
  try {
    const u = new URL(url)
    return u.pathname + u.search
  } catch {
    return url
  }
}

const query = ref('')

function matches(r: RequestRecord, q: string): boolean {
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

// --- Browser-only: group captured requests by the tab they came from ---
interface TabGroup {
  key: string
  title: string
  url: string
  favIconUrl: string
  items: RequestRecord[]
}

const groups = computed<TabGroup[]>(() => {
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
  if (!q) return groups.value
  return groups.value
    .map((g) => ({ ...g, items: g.items.filter((r) => matches(r, q)) }))
    .filter((g) => g.items.length > 0)
})

const isEmptyFiltered = computed(() =>
  props.source === 'browser' ? filteredGroups.value.length === 0 : filteredRecords.value.length === 0
)

// Until step 9 wires up per-tab capture state, the "запись" label is a
// heuristic: the most recently active group is the one still being written to.
function isRecording(g: TabGroup): boolean {
  return store.capture.recording && filteredGroups.value[0]?.key === g.key
}

const collapsed = ref<Set<string>>(new Set())
const brokenFavicons = ref<Set<string>>(new Set())

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
  store.clearRequests(g.items.map((r) => r.id))
}

function hostnameOf(url: string): string {
  try {
    return new URL(url).hostname
  } catch {
    return ''
  }
}

function groupLabel(g: TabGroup): string {
  return g.title || hostnameOf(g.url) || 'Вкладка'
}

function groupHue(key: string): number {
  let h = 0
  for (let i = 0; i < key.length; i++) h = (h * 31 + key.charCodeAt(i)) >>> 0
  return h % 360
}
</script>

<template>
  <div class="history-panel">
    <div class="panel-head">
      <span class="panel-title">{{ source === 'browser' ? 'Перехвачено' : 'История' }}</span>
      <button class="btn" :disabled="records.length === 0" @click="clearAll">Очистить</button>
    </div>

    <!-- No empty text for the browser source: the centered BrowserEmptyState in
         the main column already explains the next step, so a second message
         here would just repeat it. -->
    <div v-if="records.length === 0 && source === 'manual'" class="empty">
      <span class="empty-title">Пока пусто</span>
      <span class="empty-hint">{{ emptyHint }}</span>
    </div>

    <div v-else-if="isEmptyFiltered" class="no-results">Ничего не найдено</div>

    <ul v-else-if="source === 'manual'" class="list">
      <li
        v-for="r in filteredRecords"
        :key="r.id"
        class="item"
        :class="{ active: r.id === activeId }"
        role="button"
        tabindex="0"
        @click="select(r.id)"
        @keydown.enter="select(r.id)"
        @keydown.space.prevent="select(r.id)"
      >
        <span class="item-method mono" :class="{ active: r.id === activeId }">{{ r.method }}</span>
        <span class="item-status" :class="statusClass(r.status)">{{ r.status }}</span>
        <span class="item-path mono" :title="r.url">{{ pathOf(r.url) }}</span>
        <span class="item-time">{{ timeLabel(r.startedAt) }}</span>
      </li>
    </ul>

    <div v-else class="list">
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
            <Icon name="chevron-right" :size="10" />
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
            <span class="recording-dot"></span>
            <span>запись</span>
          </span>
          <span class="group-count">{{ g.items.length }}</span>
          <button class="group-clear" title="Очистить эту вкладку" @click.stop="clearGroup(g)"><Icon name="xmark" :size="12" /></button>
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
            <span class="item-method mono" :class="{ active: r.id === activeId }">{{ r.method }}</span>
            <span class="item-status" :class="statusClass(r.status)">{{ r.status }}</span>
            <span class="item-path mono" :title="r.url">{{ pathOf(r.url) }}</span>
            <span class="item-time">{{ timeLabel(r.startedAt) }}</span>
          </li>
        </ul>
      </section>
    </div>

    <div v-if="records.length > 0" class="panel-filter-dock">
      <div class="panel-filter-fade"></div>
      <div class="panel-filter">
        <input v-model="query" class="filter-input" placeholder="Фильтр по ссылке, методу, статусу…" spellcheck="false" />
      </div>
      <div class="panel-filter-backdrop"></div>
    </div>
  </div>
</template>

<style scoped>
@reference "../style.css";

.history-panel {
  @apply relative flex flex-col h-full min-h-0 bg-bg-panel border-r border-border;
}

.panel-head {
  @apply flex items-center justify-between h-12 px-3 border-b border-border;
}

.panel-title {
  @apply text-[13px] font-semibold;
}

/* The filter bar docks to the bottom of the panel instead of the top, so it
   stays within reach next to the resize handle — level with the "Проверить
   обновления" row at the bottom of the main sidebar, not flush against the
   window edge. It floats over the list — a gradient fades list items to
   transparent as they scroll under it, rather than the bar just clipping
   them off with a hard edge. */
.panel-filter-dock {
  @apply absolute left-0 right-0 bottom-0 flex flex-col pointer-events-none;
}

/* A smoothstep curve (3t²-2t³) rather than a hand-picked handful of stops —
   it has zero slope at both ends, so the fade eases in from "list" and
   eases out into "solid" with no visible kink or seam anywhere along it. */
.panel-filter-fade {
  @apply h-8;
  background: linear-gradient(
    to bottom,
    color-mix(in srgb, var(--bg-panel) 0%, transparent) 0%,
    color-mix(in srgb, var(--bg-panel) 3%, transparent) 10%,
    color-mix(in srgb, var(--bg-panel) 10%, transparent) 20%,
    color-mix(in srgb, var(--bg-panel) 22%, transparent) 30%,
    color-mix(in srgb, var(--bg-panel) 35%, transparent) 40%,
    color-mix(in srgb, var(--bg-panel) 50%, transparent) 50%,
    color-mix(in srgb, var(--bg-panel) 65%, transparent) 60%,
    color-mix(in srgb, var(--bg-panel) 78%, transparent) 70%,
    color-mix(in srgb, var(--bg-panel) 90%, transparent) 80%,
    color-mix(in srgb, var(--bg-panel) 97%, transparent) 90%,
    var(--bg-panel) 100%
  );
}

.panel-filter {
  @apply pointer-events-auto flex items-center py-1.5 px-3 bg-bg-panel;
}

/* Fills the gap between the filter row and the panel's true bottom edge
   (kept level with the sidebar's "Проверить обновления" row) with solid
   background, so list items never show through underneath the input. */
.panel-filter-backdrop {
  @apply h-2 bg-bg-panel;
}

.filter-input {
  @apply w-full py-2.5 px-2.5 rounded-md text-xs outline-none;
  border: 1px solid var(--border);
  background: var(--bg-inset);
  color: var(--text);
  transition: border-color 0.12s ease, box-shadow 0.12s ease;
}

.filter-input:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.no-results {
  @apply pt-4 px-4 pb-23 text-center text-text-tertiary text-xs;
}

.empty {
  @apply flex-1 flex flex-col items-center justify-center gap-2 p-4 text-center;
}

.empty-title {
  @apply text-[15px] font-semibold text-text-secondary;
}

.empty-hint {
  @apply text-xs text-text-tertiary max-w-[220px];
}

.list {
  @apply flex-1 min-h-0 overflow-auto pt-1.5 px-1.5 pb-23;
}

.group {
  @apply mb-1;
}

.group-head {
  @apply flex items-center gap-[7px] w-full py-1.5 px-1.5 border-none rounded-md bg-transparent text-text text-xs text-left cursor-pointer select-none;
  --wails-draggable: no-drag;
}

.group-head:hover {
  @apply bg-bg-hover;
}

.caret {
  @apply inline-flex items-center justify-center flex-none w-3 h-3 text-text-secondary;
  transition: transform 0.12s ease;
}

.caret.open {
  @apply rotate-90;
}

.favicon {
  @apply flex-none w-[18px] h-[18px] rounded-sm object-contain;
  border: 1px solid var(--border);
  background: var(--card);
}

.avatar {
  @apply flex-none w-[18px] h-[18px] rounded-sm text-white text-[10px] font-semibold inline-flex items-center justify-center leading-none uppercase;
}

.group-title {
  @apply flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap font-medium;
}

.recording-label {
  @apply flex-none inline-flex items-center gap-1 text-[10.5px] text-red;
}

.recording-dot {
  @apply w-1.5 h-1.5 rounded-full flex-none;
  background: var(--red);
}

.group-count {
  @apply flex-none h-[18px] leading-[18px] text-[11px] text-text-tertiary;
  font-variant-numeric: tabular-nums;
}

.group-clear {
  @apply flex-none w-[18px] h-[18px] border-none rounded-sm bg-transparent text-text-tertiary cursor-pointer flex items-center justify-center p-0 opacity-55;
  transition: opacity 0.12s ease, color 0.12s ease, background 0.12s ease;
}

.group-head:hover .group-clear {
  @apply opacity-100;
}

.group-clear:hover {
  color: var(--red);
  background: var(--red-soft);
}

.group-items {
  @apply list-none m-0 pt-0.5 pr-0 pb-0.5 pl-3.5;
}

/* A single, dense row: method → status → path → time. The duration was
   dropped here because the response header already shows it; the host was
   dropped from the path because it's redundant with the group header / URL. */
.item {
  @apply flex items-center gap-2 py-[7px] px-2.5 rounded-md cursor-pointer mb-px;
}

.item:hover {
  @apply bg-bg-hover;
}

.item.active {
  @apply bg-accent-soft;
}

.item-method {
  @apply flex-none min-w-[48px] text-[10.5px] font-semibold text-text-secondary;
  font-family: var(--mono);
}

.item-method.active {
  color: var(--accent);
}

.item-status {
  @apply flex-none inline-flex items-center px-1.5 py-px rounded-sm text-[10px] font-semibold;
  font-variant-numeric: tabular-nums;
}

.item-path {
  @apply flex-1 min-w-0 text-[11.5px] text-text overflow-hidden text-ellipsis whitespace-nowrap;
  font-family: var(--mono);
}

.item-time {
  @apply flex-none text-[10.5px] text-text-tertiary;
  font-variant-numeric: tabular-nums;
}
</style>
