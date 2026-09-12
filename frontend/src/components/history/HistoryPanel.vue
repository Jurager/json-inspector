<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Icon from '../ui/Icon.vue'
import { Button, IconButton } from '../ui/button'
import { Input } from '../ui/input'
import { useRequestsStore } from '../../stores/requests'
import type { RequestRecord } from '../../lib/types'
import { statusClass } from '../../lib/json'

// One component for both sources — same records, filtered differently (and grouped by tab for the browser one) — so their wording and styling can't drift apart.
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

// Path + query only: the host already shows in the group header or the command line, and the full URL stays in the row's title.
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
  if (dayMs === today.getTime()) return 'Сегодня'
  if (dayMs === yesterday.getTime()) return 'Вчера'
  return d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' })
}

const manualGroups = computed(() => {
  const out: { label: string; items: RequestRecord[] }[] = []
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

// "запись" is a heuristic: the most recently active group is assumed to be the one still being written to.
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

// A deep link's tab may have nothing yet, but the link is clicked right after the page loads and the first request lands a moment later — so the request is kept.
function applyDeepLink() {
  if (props.source !== 'browser') return
  const tabId = store.focusTabId
  if (tabId == null) return
  const g = groups.value.find((x) => x.key === String(tabId))
  if (!g || g.items.length === 0) return
  const next = new Set(collapsed.value)
  next.delete(g.key)
  collapsed.value = next
  // Items are newest-first, so the first one is the request just made.
  store.selectBrowser(g.items[0].id)
  store.focusTabId = null
}

// Watches the source too — the link both switches the rail and asks for a tab — and runs immediately, because the panel isn't rendered while nothing has been captured.
watch(() => [store.focusTabId, store.requests.length, props.source] as const, applyDeepLink, {
  immediate: true,
})
</script>

<template>
  <div class="history-panel">
    <div class="panel-head">
      <span class="panel-title">{{ source === 'browser' ? 'Перехвачено' : 'История' }}</span>
      <Button variant="quiet" :disabled="records.length === 0" @click="clearAll">Очистить</Button>
    </div>

    <!-- No empty text for the browser source: BrowserEmptyState in the main column already explains
         the next step.
    -->
    <div v-if="records.length === 0 && source === 'manual'" class="empty">
      <span class="empty-title">Пока пусто</span>
      <span class="empty-hint">{{ emptyHint }}</span>
    </div>

    <div v-else-if="records.length > 0 && isEmptyFiltered" class="no-results">Ничего не найдено</div>

    <ul v-else-if="source === 'manual'" class="list">
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
          <span class="item-method mono" :class="{ active: r.id === activeId }">{{ r.method }}</span>
          <span class="item-status" :class="statusClass(r.status)">{{ r.status }}</span>
          <span class="item-path mono" :title="r.url">{{ pathOf(r.url) }}</span>
          <span class="item-time">{{ timeLabel(r.startedAt) }}</span>
        </li>
      </template>
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
          <IconButton variant="danger" size="sm" hint="Очистить эту вкладку" @click.stop="clearGroup(g)"><Icon name="xmark" :size="12" /></IconButton>
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
        <Input v-model="query" size="sm" class="w-full" placeholder="Фильтр по ссылке, методу, статусу…" spellcheck="false" />
      </div>
      <div class="panel-filter-backdrop"></div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.history-panel {
  @apply relative flex flex-col h-full min-h-0 bg-bg-panel border-r border-border;
}

.panel-head {
  @apply flex items-center justify-between h-10 px-2 pl-3.5 border-b border-border;
}

.panel-title {
  @apply text-xs font-semibold text-text-secondary;
}

.date-sep {
  @apply text-[10px] uppercase tracking-[0.08em] text-text-tertiary pt-1.5 px-2 pb-1;
  font-family: var(--mono);
}

/* Docked to the bottom, level with the sidebar's "Проверить обновления" row — within reach of the
   resize handle. It floats over the list, a gradient fading items under it rather than a hard
   clip. */
.panel-filter-dock {
  @apply absolute left-0 right-0 bottom-0 flex flex-col pointer-events-none;
}

/* A smoothstep curve (3t²-2t³): zero slope at both ends, so the fade eases in and out with no
   visible kink. */
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

/* Solid background down to the panel's true bottom edge, so list items never show through under
   the input. */
.panel-filter-backdrop {
  @apply h-2 bg-bg-panel;
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

/* The button fades in with its row, so the rule reaches in from the group header — Vue puts the
   parent's scope attribute on a child's root, which is why the selector matches. */
.group-head :deep(.icon-btn) {
  opacity: 0.55;
}

.group-head:hover :deep(.icon-btn) {
  opacity: 1;
}

.group-items {
  @apply list-none m-0 pt-0.5 pr-0 pb-0.5 pl-3.5;
}

/* One dense row: method → status → path → time. Duration lives in the response header, and the
   host is redundant with the group header / URL. */
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
