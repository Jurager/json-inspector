<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, type Ref } from 'vue'
import { useRequestsStore } from './stores/requests'
import RequestBuilder from './components/RequestBuilder.vue'
import ResponseViewer from './components/ResponseViewer.vue'
import HistoryPanel from './components/HistoryPanel.vue'
import Updater from './components/Updater.vue'
import AboutModal from './components/AboutModal.vue'
import Icon from './components/Icon.vue'
import logoUrl from './assets/logo.svg'
import {
  EventsOn,
  Environment,
  WindowMinimise,
  WindowToggleMaximise,
  WindowIsMaximised,
  Quit,
} from '../wailsjs/runtime/runtime'
import { ToggleMaximize } from '../wailsjs/go/main/App'

const store = useRequestsStore()

const HISTORY_KEY = 'ji-history-v1'
const MAX_HISTORY = 200

const aboutOpen = ref(false)

// Windows and Linux have no equivalent of macOS's hidden-inset title bar, so
// the window there runs frameless and this titlebar draws its own icon,
// title and caption buttons (minimize/maximize/close) to look native.
const useCustomTitlebar = ref(false)
const isMaximised = ref(false)

async function refreshMaximised() {
  try {
    isMaximised.value = await WindowIsMaximised()
  } catch {
    // ignore — runtime not ready yet
  }
}

function openBrowser() {
  store.activeView = 'browser'
  store.markBrowserRead()
}

// One side panel, shared by both tabs — it swaps content (history vs.
// browser-captured list) depending on store.activeView, so its width is a
// single piece of state and resizing it on either tab carries over to the
// other.
function makeSideResizer(width: Ref<number>, min: number, max: number) {
  let state: { startX: number; startWidth: number } | null = null
  return {
    start(e: MouseEvent) {
      state = { startX: e.clientX, startWidth: width.value }
      document.body.style.cursor = 'col-resize'
      document.body.style.userSelect = 'none'
    },
    move(e: MouseEvent) {
      if (!state) return
      const delta = e.clientX - state.startX
      width.value = Math.min(max, Math.max(min, state.startWidth + delta))
    },
    stop() {
      state = null
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    },
  }
}

const sideWidth = ref(300)
const sideResize = makeSideResizer(sideWidth, 220, 560)

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
  tabId?: number
  favIconUrl?: string
}

const offs: (() => void)[] = []

onMounted(() => {
  Environment()
    .then((env) => {
      useCustomTitlebar.value = env.platform === 'windows' || env.platform === 'linux'
      if (useCustomTitlebar.value) {
        refreshMaximised()
        window.addEventListener('resize', refreshMaximised)
      }
    })
    .catch(() => {
      // ignore — default to the macOS-style titlebar
    })

  // Restore request history from the previous session.
  try {
    const raw = localStorage.getItem(HISTORY_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (Array.isArray(parsed)) store.hydrate(parsed)
    }
  } catch {
    // ignore corrupt storage
  }

  // Persist request history (debounced).
  let saveTimer: ReturnType<typeof setTimeout> | null = null
  offs.push(
    store.$subscribe((_m, state) => {
      if (saveTimer) clearTimeout(saveTimer)
      saveTimer = setTimeout(() => {
        try {
          localStorage.setItem(HISTORY_KEY, JSON.stringify(state.requests.slice(0, MAX_HISTORY)))
        } catch {
          // ignore quota errors
        }
      }, 300)
    })
  )

  offs.push(
    EventsOn('captured-request', (c: Captured) => {
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
        tabId: c.tabId,
        favIconUrl: c.favIconUrl,
      })
    })
  )
  offs.push(
    EventsOn('show-about', () => {
      aboutOpen.value = true
    })
  )

  window.addEventListener('mousemove', sideResize.move)
  window.addEventListener('mouseup', sideResize.stop)
})

onBeforeUnmount(() => {
  offs.forEach((off) => off())
  window.removeEventListener('mousemove', sideResize.move)
  window.removeEventListener('mouseup', sideResize.stop)
  window.removeEventListener('resize', refreshMaximised)
})
</script>

<template>
  <div class="app">
    <header class="titlebar" :class="{ 'titlebar-custom': useCustomTitlebar }" @dblclick="ToggleMaximize">
      <template v-if="useCustomTitlebar">
        <button class="titlebar-appicon" @click="aboutOpen = true" title="О программе">
          <img :src="logoUrl" alt="" class="titlebar-logo" draggable="false" />
          <span class="titlebar-title">JSON Inspector</span>
        </button>
        <div class="titlebar-spacer"></div>
        <div class="titlebar-controls">
          <button class="cap-btn" title="Свернуть" @click="WindowMinimise">
            <span class="cap-icon cap-icon-minus"></span>
          </button>
          <button class="cap-btn" title="Развернуть" @click="WindowToggleMaximise">
            <span v-if="!isMaximised" class="cap-icon cap-icon-square"></span>
            <span v-else class="cap-icon cap-icon-restore">
              <span class="cap-icon-restore-back"></span>
              <span class="cap-icon-restore-front"></span>
            </span>
          </button>
          <button class="cap-btn cap-close" title="Закрыть" @click="Quit">
            <span class="cap-icon cap-icon-close">
              <span class="cap-icon-close-bar cap-icon-close-bar-1"></span>
              <span class="cap-icon-close-bar cap-icon-close-bar-2"></span>
            </span>
          </button>
        </div>
      </template>
      <span v-else class="titlebar-title">JSON Inspector</span>
    </header>

    <div class="body">
      <aside class="sidebar">
        <button
          class="nav-item"
          :class="{ active: store.activeView === 'request' }"
          @click="store.activeView = 'request'"
        >
          <span class="nav-icon"><Icon name="arrow-up-right" /></span> Запрос
        </button>
        <button
          class="nav-item"
          :class="{ active: store.activeView === 'browser' }"
          @click="openBrowser"
        >
          <span class="nav-icon"><Icon name="record" /></span> Браузер
          <span v-if="store.unreadCount > 0" class="nav-badge">{{ store.unreadCount }}</span>
        </button>

        <Updater />
      </aside>

      <main class="main">
        <div class="side-layout">
          <div class="side-panel" :style="{ width: sideWidth + 'px' }">
            <HistoryPanel :source="store.activeView === 'request' ? 'manual' : 'browser'" />
          </div>
          <div class="resize-handle" @mousedown.prevent="sideResize.start"></div>
          <div class="side-main">
            <template v-if="store.activeView === 'request'">
              <RequestBuilder />
              <ResponseViewer v-if="store.manualSelected" :record="store.manualSelected" />
              <div v-else-if="store.loading" class="empty">
                <span class="spinner spinner-lg"></span>
              </div>
              <div v-else class="empty">
                <span class="empty-title">Отправьте запрос</span>
                <span>Или нажмите «Образец», чтобы увидеть JSON:API-документ.</span>
              </div>
            </template>
            <template v-else>
              <ResponseViewer v-if="store.browserSelected" :record="store.browserSelected" />
              <div v-else class="empty">
                <span class="empty-title">Нет выбранного запроса</span>
                <span>Выберите запрос из списка слева.</span>
              </div>
            </template>
          </div>
        </div>
      </main>
    </div>
  </div>

  <AboutModal v-if="aboutOpen" @close="aboutOpen = false" />
</template>

<style scoped>
/* Both the "Запрос" view (history + builder/response) and the "Браузер"
   view (captured list + response) are the same shape — a resizable side
   panel next to a main column — so they share one set of classes. */
.side-layout {
  display: flex;
  flex: 1;
  min-height: 0;
}

.side-panel {
  flex: 0 0 auto;
  min-width: 0;
}

.resize-handle {
  flex: 0 0 5px;
  width: 5px;
  margin-left: -5px;
  position: relative;
  z-index: 1;
  cursor: col-resize;
  background: transparent;
  transition: background 0.15s ease;
}

.resize-handle:hover {
  background: var(--border-strong);
}

.side-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.side-main > :last-child {
  flex: 1;
  min-height: 0;
}
</style>
