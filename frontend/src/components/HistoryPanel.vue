<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from './Icon.vue'
import { useRequestsStore } from '../stores/requests'
import type { RequestRecord } from '../lib/types'
import { statusClass, formatDuration } from '../lib/json'

// One panel, two sources: the manually-built requests ("Запрос" tab) and the
// ones captured by the browser extension ("Браузер" tab). They're the same
// list of request/response records, just filtered differently and — for the
// browser source only — grouped by the tab they came from. Sharing one
// component keeps their wording and styling identical instead of two panels
// silently drifting apart.
const props = defineProps<{ source: 'manual' | 'browser' }>()

const store = useRequestsStore()

const emptyTitle = computed(() => (props.source === 'browser' ? 'Ничего нет' : 'Пока пусто'))
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
      <span class="panel-title">История</span>
      <button class="btn" :disabled="records.length === 0" @click="clearAll">Очистить</button>
    </div>

    <div v-if="records.length === 0" class="empty">
      <span class="empty-title">{{ emptyTitle }}</span>
      <span class="empty-hint">{{ emptyHint }}</span>
    </div>

    <div v-else-if="isEmptyFiltered" class="no-results">Ничего не найдено</div>

    <ul v-else-if="source === 'manual'" class="list">
      <li
        v-for="r in filteredRecords"
        :key="r.id"
        class="item"
        :class="{ active: r.id === activeId }"
        @click="select(r.id)"
      >
        <div class="item-top">
          <span class="badge badge-method">{{ r.method }}</span>
          <span class="badge" :class="statusClass(r.status)">{{ r.status }}</span>
          <span class="item-time">{{ timeLabel(r.startedAt) }}</span>
          <span class="item-dur">{{ formatDuration(r.durationMs) }}</span>
        </div>
        <div class="item-url mono">{{ r.url }}</div>
      </li>
    </ul>

    <div v-else class="list">
      <section v-for="g in filteredGroups" :key="g.key" class="group">
        <div class="group-head" role="button" tabindex="0" @click="toggleGroup(g.key)">
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
          <span class="group-count">{{ g.items.length }}</span>
          <button class="group-clear" title="Очистить эту вкладку" @click.stop="clearGroup(g)"><Icon name="xmark" :size="12" /></button>
        </div>

        <ul v-show="!collapsed.has(g.key)" class="group-items">
          <li
            v-for="r in g.items"
            :key="r.id"
            class="item"
            :class="{ active: r.id === activeId }"
            @click="select(r.id)"
          >
            <div class="item-top">
              <span class="badge badge-method">{{ r.method }}</span>
              <span class="badge" :class="statusClass(r.status)">{{ r.status }}</span>
              <span class="item-time">{{ timeLabel(r.startedAt) }}</span>
              <span class="item-dur">{{ formatDuration(r.durationMs) }}</span>
            </div>
            <div class="item-url mono">{{ r.url }}</div>
          </li>
        </ul>
      </section>
    </div>

    <div v-if="records.length > 0" class="panel-filter-dock">
      <div class="panel-filter-fade"></div>
      <div class="panel-filter">
        <input v-model="query" class="filter-input mono" placeholder="Фильтр по методу, статусу, URL…" spellcheck="false" />
      </div>
      <div class="panel-filter-backdrop"></div>
    </div>
  </div>
</template>

<style scoped>
.history-panel {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  background: var(--bg-panel);
  border-right: 1px solid var(--border);
}

.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 48px;
  padding: 0 12px;
  border-bottom: 1px solid var(--border);
}

.panel-title {
  font-size: 13px;
  font-weight: 600;
}

/* The filter bar docks to the bottom of the panel instead of the top, so it
   stays within reach next to the resize handle — level with the "Проверить
   обновления" row at the bottom of the main sidebar, not flush against the
   window edge. It floats over the list — a gradient fades list items to
   transparent as they scroll under it, rather than the bar just clipping
   them off with a hard edge. */
.panel-filter-dock {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  flex-direction: column;
  pointer-events: none;
}

/* A smoothstep curve (3t²-2t³) rather than a hand-picked handful of stops —
   it has zero slope at both ends, so the fade eases in from "list" and
   eases out into "solid" with no visible kink or seam anywhere along it. */
.panel-filter-fade {
  height: 32px;
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
  pointer-events: auto;
  display: flex;
  align-items: center;
  padding: 6px 12px;
  background: var(--bg-panel);
}

/* Fills the gap between the filter row and the panel's true bottom edge
   (kept level with the sidebar's "Проверить обновления" row) with solid
   background, so list items never show through underneath the input. */
.panel-filter-backdrop {
  height: 8px;
  background: var(--bg-panel);
}

.filter-input {
  width: 100%;
  padding: 9px 10px;
  border-radius: 7px;
  border: 1px solid var(--border);
  background: var(--bg-inset);
  color: var(--text);
  font-size: 12px;
  outline: none;
  transition: border-color 0.12s ease, box-shadow 0.12s ease;
}

.filter-input:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.no-results {
  padding: 16px 16px 92px;
  text-align: center;
  color: var(--text-tertiary);
  font-size: 12px;
}

.empty {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 16px;
  text-align: center;
}

.empty-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-secondary);
}

.empty-hint {
  font-size: 12px;
  color: var(--text-tertiary);
  max-width: 220px;
}

.list {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 6px 6px 92px;
}

.group {
  margin-bottom: 4px;
}

.group-head {
  display: flex;
  align-items: center;
  gap: 7px;
  width: 100%;
  padding: 5px 6px;
  border: none;
  border-radius: 7px;
  background: transparent;
  color: var(--text);
  font-size: 12px;
  text-align: left;
  cursor: pointer;
  user-select: none;
  --wails-draggable: no-drag;
}

.group-head:hover {
  background: var(--bg-hover);
}

.caret {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 12px;
  width: 12px;
  height: 12px;
  color: var(--text-secondary);
  transition: transform 0.12s ease;
}

.caret.open {
  transform: rotate(90deg);
}

.favicon {
  flex: 0 0 auto;
  width: 18px;
  height: 18px;
  border-radius: 4px;
  border: 1px solid var(--border);
  background: var(--card);
  object-fit: contain;
}

.avatar {
  flex: 0 0 auto;
  width: 18px;
  height: 18px;
  border-radius: 4px;
  color: #fff;
  font-size: 10px;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
  text-transform: uppercase;
}

.group-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}

.group-count {
  flex: 0 0 auto;
  height: 18px;
  line-height: 18px;
  font-size: 11px;
  color: var(--text-tertiary);
  font-variant-numeric: tabular-nums;
}

.group-clear {
  flex: 0 0 auto;
  width: 18px;
  height: 18px;
  border: none;
  border-radius: 5px;
  background: transparent;
  color: var(--text-tertiary);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  opacity: 0.55;
  transition: opacity 0.12s ease, color 0.12s ease, background 0.12s ease;
}

.group-head:hover .group-clear {
  opacity: 1;
}

.group-clear:hover {
  color: var(--red);
  background: var(--red-soft);
}

.group-items {
  list-style: none;
  margin: 0;
  padding: 2px 0 2px 16px;
}

.item {
  padding: 7px 8px;
  border-radius: 7px;
  cursor: pointer;
  margin-bottom: 1px;
}

.item:hover {
  background: var(--bg-hover);
}

.item.active {
  background: var(--accent-soft);
}

.item-top {
  display: flex;
  align-items: center;
  gap: 6px;
}

.item-time {
  margin-left: auto;
  color: var(--text-tertiary);
  font-size: 11px;
}

.item-dur {
  color: var(--text-tertiary);
  font-size: 11px;
}

.item-url {
  margin-top: 3px;
  font-size: 11px;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
