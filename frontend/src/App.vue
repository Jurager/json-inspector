<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRequestsStore } from './stores/requests'
import RequestBuilder from './components/RequestBuilder.vue'
import ResponseViewer from './components/ResponseViewer.vue'
import BrowserPanel from './components/BrowserPanel.vue'
import Updater from './components/Updater.vue'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { ToggleMaximize } from '../wailsjs/go/main/App'

const store = useRequestsStore()

function openBrowser() {
  store.activeView = 'browser'
  store.markBrowserRead()
}

const browserSideWidth = ref(300)
let resizeState: { startX: number; startWidth: number } | null = null

function startResize(e: MouseEvent) {
  resizeState = { startX: e.clientX, startWidth: browserSideWidth.value }
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
}

function onResizeMove(e: MouseEvent) {
  if (!resizeState) return
  const delta = e.clientX - resizeState.startX
  browserSideWidth.value = Math.min(600, Math.max(180, resizeState.startWidth + delta))
}

function stopResize() {
  resizeState = null
  document.body.style.cursor = ''
  document.body.style.userSelect = ''
}

interface Captured {
  method: string
  url: string
  requestHeaders?: Record<string, string>
  requestBody?: string
  status: number
  statusText?: string
  responseHeaders?: Record<string, string>
  responseBody?: string
  durationMs?: number
  tabTitle?: string
  tabURL?: string
}

let off: (() => void) | null = null

onMounted(() => {
  off = EventsOn('captured-request', (c: Captured) => {
    const headers = c.responseHeaders ?? {}
    store.addCaptured({
      method: c.method,
      url: c.url,
      requestHeaders: c.requestHeaders ?? {},
      requestBody: c.requestBody ?? '',
      status: c.status,
      statusText: c.statusText ?? '',
      responseHeaders: headers,
      responseBody: c.responseBody ?? '',
      durationMs: c.durationMs ?? 0,
      contentType: headers['content-type'] ?? headers['Content-Type'] ?? '',
      tabTitle: c.tabTitle,
      tabURL: c.tabURL,
    })
  })

  window.addEventListener('mousemove', onResizeMove)
  window.addEventListener('mouseup', stopResize)
})

onBeforeUnmount(() => {
  off?.()
  window.removeEventListener('mousemove', onResizeMove)
  window.removeEventListener('mouseup', stopResize)
})
</script>

<template>
  <div class="app">
    <header class="titlebar" @dblclick="ToggleMaximize">
      <span class="titlebar-title">JSON Inspector</span>
    </header>

    <div class="body">
      <aside class="sidebar">
        <button
          class="nav-item"
          :class="{ active: store.activeView === 'request' }"
          @click="store.activeView = 'request'"
        >
          <span class="nav-icon">↗</span> Запрос
        </button>
        <button
          class="nav-item"
          :class="{ active: store.activeView === 'browser' }"
          @click="openBrowser"
        >
          <span class="nav-icon">◉</span> Браузер
          <span v-if="store.unreadCount > 0" class="nav-badge">{{ store.unreadCount }}</span>
        </button>

        <Updater />
      </aside>

      <main class="main">
        <template v-if="store.activeView === 'request'">
          <div class="request-view">
            <RequestBuilder />
            <ResponseViewer v-if="store.selected" :record="store.selected" />
            <div v-else-if="store.loading" class="empty">
              <span class="spinner spinner-lg"></span>
            </div>
            <div v-else class="empty">
              <span class="empty-title">Отправьте запрос</span>
              <span>Или нажмите «Образец», чтобы увидеть JSON:API-документ.</span>
            </div>
          </div>
        </template>

        <template v-else>
          <div class="browser-layout">
            <div class="browser-side" :style="{ width: browserSideWidth + 'px' }"><BrowserPanel /></div>
            <div class="resize-handle" @mousedown.prevent="startResize"></div>
            <div class="browser-main">
              <ResponseViewer v-if="store.selected" :record="store.selected" />
              <div v-else class="empty">
                <span class="empty-title">Нет выбранного запроса</span>
                <span>Выберите запрос из списка слева.</span>
              </div>
            </div>
          </div>
        </template>
      </main>
    </div>
  </div>
</template>

<style scoped>
.request-view {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

.request-view .builder {
  flex: 0 0 auto;
}

.request-view > :last-child {
  flex: 1;
  min-height: 0;
}

.browser-layout {
  display: flex;
  flex: 1;
  min-height: 0;
}

.browser-side {
  flex: 0 0 auto;
  min-width: 0;
}

.resize-handle {
  flex: 0 0 5px;
  width: 5px;
  cursor: col-resize;
  background: transparent;
  transition: background 0.15s ease;
}

.resize-handle:hover {
  background: var(--border-strong);
}

.browser-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.browser-main > :first-child {
  flex: 1;
  min-height: 0;
}
</style>
