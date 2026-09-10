<script setup lang="ts">
import { computed } from 'vue'
import { useRequestsStore } from '../stores/requests'
import { statusClass, formatDuration } from '../lib/json'

const store = useRequestsStore()

const captured = computed(() => store.requests.filter((r) => r.source === 'browser'))

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

    <div v-if="captured.length === 0" class="empty">
      <span class="empty-title">Пока пусто</span>
      <span class="empty-hint">Установите расширение и откройте сайт — перехваченные запросы появятся здесь.</span>
    </div>

    <ul v-else class="list">
      <li
        v-for="r in captured"
        :key="r.id"
        class="item"
        :class="{ active: r.id === store.selectedId }"
        @click="store.select(r.id)"
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
  padding: 10px 12px;
  border-bottom: 1px solid var(--border);
}

.browser-title {
  font-size: 13px;
  font-weight: 600;
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
  list-style: none;
  margin: 0;
  padding: 6px;
  overflow: auto;
  flex: 1;
}

.item {
  padding: 8px 10px;
  border-radius: 8px;
  cursor: pointer;
  margin-bottom: 2px;
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
  margin-top: 4px;
  font-size: 11px;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
