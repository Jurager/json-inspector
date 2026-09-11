<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRequestsStore } from './stores/requests'
import RequestBuilder from './components/RequestBuilder.vue'
import ResponseViewer from './components/ResponseViewer.vue'
import HistoryPanel from './components/HistoryPanel.vue'
import AboutModal from './components/AboutModal.vue'
import StatusBar from './components/StatusBar.vue'
import CaptureBar from './components/CaptureBar.vue'
import BrowserEmptyState from './components/BrowserEmptyState.vue'
import Icon from './components/Icon.vue'
import logoUrl from './assets/logo.svg'
import { buildSampleRecord } from './lib/sample'
import { useCustomTitlebar } from './lib/platform'
import { makeSideResizer } from './lib/resize'
import {
  EventsOn,
  WindowMinimise,
  WindowToggleMaximise,
  WindowIsMaximised,
  Quit,
} from '../wailsjs/runtime/runtime'
import { ToggleMaximize, CheckForUpdates, UpdateNow } from '../wailsjs/go/main/App'

const store = useRequestsStore()

const HISTORY_KEY = 'ji-history-v1'
const UI_KEY = 'ji-ui-v1'
const MAX_HISTORY = 200

const aboutOpen = ref(false)
const isMaximised = ref(false)

interface UpdateInfo {
  available: boolean
  current: string
  latest: string
}

const checking = ref(false)
const updating = ref(false)
const update = ref<UpdateInfo | null>(null)
const updateModalOpen = ref(false)
const toast = ref('')
const toastType = ref<'info' | 'error'>('info')
let toastTimer: ReturnType<typeof setTimeout> | null = null

function openUpdate() {
  updateModalOpen.value = true
}

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

function loadSample() {
  store.activeView = 'request'
  store.add(buildSampleRecord())
}

function showToast(msg: string, type: 'info' | 'error' = 'info') {
  toast.value = msg
  toastType.value = type
  if (toastTimer) clearTimeout(toastTimer)
  toastTimer = setTimeout(() => (toast.value = ''), 5000)
}

async function checkUpdates() {
  if (checking.value) return
  checking.value = true
  try {
    const u = await CheckForUpdates()
    if (u.available) {
      update.value = u
      // A manual check gives immediate feedback, unlike the silent startup
      // check which only lights up the status-bar link.
      updateModalOpen.value = true
    } else showToast(`У вас последняя версия (${u.latest})`)
  } catch {
    showToast('Не удалось проверить обновления', 'error')
  } finally {
    checking.value = false
  }
}

async function doUpdate() {
  if (!update.value || updating.value) return
  updating.value = true
  const v = update.value.latest
  try {
    await UpdateNow(v)
  } catch (e) {
    showToast(`Не удалось обновиться: ${e}`, 'error')
  } finally {
    updating.value = false
  }
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
const sideWidth = ref(300)
const sideResize = makeSideResizer(sideWidth, 220, 560)

// With no captured requests at all, the browser view drops the list entirely
// and shows the onboarding full-width — the design's "одно пустое состояние".
const browserEmpty = computed(
  () => store.activeView === 'browser' && !store.requests.some((r) => r.source === 'browser')
)

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

  // Restore the inspector's visibility and width.
  try {
    const raw = localStorage.getItem(UI_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (parsed && typeof parsed.open === 'boolean') store.setInspector({ open: parsed.open })
      if (parsed && typeof parsed.width === 'number') store.setInspector({ width: parsed.width })
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
          localStorage.setItem(UI_KEY, JSON.stringify({ open: state.inspector.open, width: state.inspector.width }))
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
    EventsOn('capture-state', (s: { recording: boolean; tabs: number }) => {
      store.setCaptureState({ connected: true, recording: s.recording, tabs: s.tabs })
    })
  )
  offs.push(
    EventsOn('capture-disconnected', () => {
      store.setCaptureState({ connected: false, recording: false, tabs: 0 })
    })
  )
  offs.push(
    EventsOn('show-about', () => {
      aboutOpen.value = true
    })
  )
  offs.push(
    EventsOn('update-available', (u: UpdateInfo) => {
      update.value = u
    })
  )
  offs.push(
    EventsOn('update-up-to-date', (u: UpdateInfo) => {
      showToast(`У вас последняя версия (${u.latest})`)
    })
  )
  offs.push(
    EventsOn('update-error', (msg: string) => {
      showToast(msg, 'error')
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
            <button class="menu-item" @click="loadSample(); railMenuOpen = false">
              <Icon name="sparkles" :size="14" /> Загрузить образец
            </button>
            <button class="menu-item" @click="aboutOpen = true; railMenuOpen = false">
              <Icon name="info" :size="14" /> О программе
            </button>
            <button class="menu-item" @click="checkUpdates(); railMenuOpen = false">
              <Icon name="arrow-down" :size="14" /> {{ checking ? 'Проверка…' : 'Проверить обновления' }}
            </button>
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
          <div v-if="!browserEmpty" class="side-panel" :style="{ width: sideWidth + 'px' }">
            <HistoryPanel :source="store.activeView === 'request' ? 'manual' : 'browser'" />
          </div>
          <div v-if="!browserEmpty" class="resize-handle" @mousedown.prevent="sideResize.start"></div>
          <div class="side-main">
            <template v-if="store.activeView === 'request'">
              <RequestBuilder />
              <ResponseViewer v-if="store.manualSelected" :record="store.manualSelected" />
              <div v-else-if="store.loading" class="empty">
                <span class="spinner spinner-lg"></span>
              </div>
              <div v-else class="empty">
                <span class="empty-title">Отправьте запрос</span>
                <span>Или загрузите образец JSON:API из меню.</span>
              </div>
            </template>
            <template v-else>
              <CaptureBar />
              <ResponseViewer v-if="store.browserSelected" :record="store.browserSelected" />
              <BrowserEmptyState v-else />
            </template>
          </div>
        </div>
      </main>
    </div>
    <StatusBar :update="update" @open-update="openUpdate" />
  </div>

  <AboutModal v-if="aboutOpen" @close="aboutOpen = false" />

  <transition name="fade">
    <div v-if="toast" class="toast" :class="toastType">{{ toast }}</div>
  </transition>

  <div v-if="update && updateModalOpen" class="modal-overlay" @click.self="updateModalOpen = false">
    <div class="modal">
      <div class="modal-title">Доступна новая версия</div>
      <div class="modal-body">
        Версия <b>{{ update.latest }}</b> (у вас {{ update.current }}).<br />
        Обновить сейчас? Приложение перезапустится.
      </div>
      <div class="modal-actions">
        <button class="btn" @click="updateModalOpen = false">Позже</button>
        <button class="btn btn-primary" :disabled="updating" @click="doUpdate">
          {{ updating ? 'Обновление…' : 'Обновить' }}
        </button>
      </div>
    </div>
  </div>
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

.toast {
  @apply fixed bottom-4 left-1/2 px-4 py-2 rounded-lg text-xs z-2000 max-w-[80%];
  transform: translateX(-50%);
  background: var(--bg-panel);
  border: 1px solid var(--border);
  box-shadow: var(--shadow);
}

.toast.error {
  @apply text-red;
}

.toast.info {
  @apply text-text;
}

.modal-overlay {
  @apply fixed inset-0 flex items-center justify-center z-1500;
  background: rgba(0, 0, 0, 0.4);
}

.modal {
  @apply w-90 max-w-[90%] rounded-xl p-4.5;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  box-shadow: var(--shadow);
}

.modal-title {
  @apply text-[15px] font-semibold mb-2;
}

.modal-body {
  @apply text-[13px] text-text-secondary mb-4;
}

.modal-actions {
  @apply flex justify-end gap-2;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
