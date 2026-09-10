<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch, type Ref } from 'vue'
import { useRequestsStore } from './stores/requests'
import RequestBuilder from './components/RequestBuilder.vue'
import ResponseViewer from './components/ResponseViewer.vue'
import HistoryPanel from './components/HistoryPanel.vue'
import Updater from './components/Updater.vue'
import AboutModal from './components/AboutModal.vue'
import Icon from './components/Icon.vue'
import logoUrl from './assets/logo.svg'
import { useCustomTitlebar } from './lib/platform'
import {
  EventsOn,
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

// Rail menu (hamburger at the top of the icon rail) — app-level actions that
// don't belong to either tab. Closes on an outside click, same pattern as
// the copy/export dropdowns elsewhere in the app.
const railMenuOpen = ref(false)
const railMenuEl = ref<HTMLElement | null>(null)

function onDocClick(e: MouseEvent) {
  if (railMenuEl.value && !railMenuEl.value.contains(e.target as Node)) {
    railMenuOpen.value = false
  }
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

// useCustomTitlebar resolves asynchronously (it waits on Wails' own
// Environment() call, in lib/platform.ts) — watch it rather than checking
// once, so the maximize-state listener still gets wired up once it lands.
watch(
  useCustomTitlebar,
  (custom) => {
    if (custom) {
      refreshMaximised()
      window.addEventListener('resize', refreshMaximised)
    }
  },
  { immediate: true }
)

onMounted(() => {
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
  document.addEventListener('click', onDocClick)
})

onBeforeUnmount(() => {
  offs.forEach((off) => off())
  window.removeEventListener('mousemove', sideResize.move)
  window.removeEventListener('mouseup', sideResize.stop)
  window.removeEventListener('resize', refreshMaximised)
  document.removeEventListener('click', onDocClick)
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
        <div ref="railMenuEl" class="rail-menu-wrap">
          <button class="rail-menu-btn" title="Меню" @click="railMenuOpen = !railMenuOpen">
            <Icon name="menu" :size="18" />
          </button>
          <div v-if="railMenuOpen" class="menu rail-menu-dropdown">
            <button class="menu-item" @click="aboutOpen = true; railMenuOpen = false">
              <Icon name="info" :size="14" /> О программе
            </button>
            <Updater />
          </div>
        </div>

        <button
          class="rail-item"
          :class="{ active: store.activeView === 'request' }"
          @click="store.activeView = 'request'"
        >
          <span class="rail-icon"><Icon name="arrow-up-right" :size="18" /></span>
          <span class="rail-label">Запрос</span>
        </button>
        <button
          class="rail-item"
          :class="{ active: store.activeView === 'browser' }"
          @click="openBrowser"
        >
          <span class="rail-icon">
            <Icon name="record" :size="18" />
            <span v-if="store.unreadCount > 0" class="rail-badge">{{ store.unreadCount }}</span>
          </span>
          <span class="rail-label">Браузер</span>
        </button>
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
@reference "./style.css";

/* Both the "Запрос" view (history + builder/response) and the "Браузер"
   view (captured list + response) are the same shape — a resizable side
   panel next to a main column — so they share one set of classes. */
.side-layout {
  @apply flex flex-1 min-h-0;
}

.side-panel {
  @apply flex-none min-w-0;
}

.resize-handle {
  @apply flex-none w-[5px] -ml-[5px] relative z-1 bg-transparent;
  cursor: col-resize;
  transition: background 0.15s ease;
}

.resize-handle:hover {
  background: var(--border-strong);
}

.side-main {
  @apply flex-1 min-w-0 flex flex-col;
}

.side-main > :last-child {
  @apply flex-1 min-h-0;
}
</style>
