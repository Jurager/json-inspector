<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from './Icon.vue'
import { useRequestsStore } from '../stores/requests'
import type { RequestRecord } from '../lib/types'
import { statusClass, formatDuration } from '../lib/json'

const store = useRequestsStore()

const captured = computed(() => store.requests.filter((r) => r.source === 'browser'))

interface TabGroup {
  key: string
  title: string
  url: string
  favIconUrl: string
  items: RequestRecord[]
}

const groups = computed<TabGroup[]>(() => {
  const map = new Map<string, TabGroup>()
  for (const r of captured.value) {
    const key = r.tabId != null ? String(r.tabId) : (r.tabURL || 'unknown')
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

const query = ref('')

const filteredGroups = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return groups.value
  return groups.value
    .map((g) => ({
      ...g,
      items: g.items.filter(
        (r) =>
          r.method.toLowerCase().includes(q) ||
          String(r.status).includes(q) ||
          r.url.toLowerCase().includes(q)
      ),
    }))
    .filter((g) => g.items.length > 0)
})

const collapsed = ref<Set<string>>(new Set())
const brokenFavicons = ref<Set<string>>(new Set())

function toggle(key: string) {
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

function clearTab(g: TabGroup) {
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

function timeLabel(startedAt: number): string {
  const d = new Date(startedAt)
  return d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}
</script>

<template>
  <div class="browser">
    <div class="browser-head">
      <span class="browser-title">Запросы из браузера</span>
      <button class="btn" :disabled="captured.length === 0" @click="store.clear()">Очистить</button>
    </div>

    <div v-if="captured.length > 0" class="browser-filter">
      <input v-model="query" class="filter-input mono" placeholder="Фильтр по методу, статусу, URL…" spellcheck="false" />
    </div>

    <div v-if="captured.length === 0" class="empty">
      <span class="empty-title">Ничего нет</span>
      <span class="empty-hint">Установите и активируйте расширение. Перехваченные запросы появятся здесь.</span>
    </div>

    <div v-else-if="filteredGroups.length === 0" class="no-results">
      Ничего не найдено
    </div>

    <div v-else class="list">
      <section v-for="g in filteredGroups" :key="g.key" class="group">
        <div class="group-head" role="button" tabindex="0" @click="toggle(g.key)">
          <span class="caret" :class="{ open: !collapsed.has(g.key) }">
            <svg viewBox="0 0 8 12" width="8" height="12" fill="none" aria-hidden="true">
              <path d="M1.5 1.5L6 6L1.5 10.5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </span>
          <img
            v-if="g.favIconUrl && !brokenFavicons.has(g.key)"
            class="favicon"
            :src="g.favIconUrl"
            alt=""
            @error="markBroken(g.key)"
          />
          <span
            v-else
            class="avatar"
            :style="{ background: `hsl(${groupHue(g.key)}, 58%, 45%)` }"
          >
            {{ groupLabel(g).charAt(0).toUpperCase() }}
          </span>
          <span class="group-title">{{ groupLabel(g) }}</span>
          <span class="group-count">{{ g.items.length }}</span>
          <button class="group-clear" title="Очистить эту вкладку" @click.stop="clearTab(g)"><Icon name="xmark" :size="12" /></button>
        </div>

        <ul v-show="!collapsed.has(g.key)" class="group-items">
          <li
            v-for="r in g.items"
            :key="r.id"
            class="item"
            :class="{ active: r.id === store.browserId }"
            @click="store.selectBrowser(r.id)"
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
  </div>
</template>

<style scoped>
.browser {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  background: var(--bg-panel);
  border-right: 1px solid var(--border);
}

.browser-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 48px;
  padding: 0 12px;
  border-bottom: 1px solid var(--border);
}

.browser-title {
  font-size: 13px;
  font-weight: 600;
}

.browser-filter {
  padding: 8px 12px;
  border-bottom: 1px solid var(--border);
}

.filter-input {
  width: 100%;
  padding: 5px 8px;
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
  padding: 16px;
  text-align: center;
  color: var(--text-tertiary);
  font-size: 12px;
}

.empty {
  flex: 1;
  padding: 16px;
  text-align: center;
}

.empty-hint {
  font-size: 12px;
  color: var(--text-tertiary);
  max-width: 220px;
}

.list {
  flex: 1;
  overflow: auto;
  padding: 6px;
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
  flex: 0 0 14px;
  width: 14px;
  height: 14px;
  font-size: 11px;
  line-height: 1;
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
  font-size: 14px;
  line-height: 1;
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
