<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRequestsStore } from '../../stores/requests'
import { makeSideResizer } from '../../lib/resize'
import HistoryPanel from '../history/HistoryPanel.vue'
import RequestBuilder from '../request/RequestBuilder.vue'
import ResponseViewer from '../response/ResponseViewer.vue'
import CaptureBar from '../browser/CaptureBar.vue'
import BrowserEmptyState from '../browser/BrowserEmptyState.vue'

const store = useRequestsStore()

// One side panel, shared by both tabs — it swaps content (history vs.
// browser-captured list) depending on store.activeView, so its width is a
// single piece of state and resizing it on either tab carries over to the
// other tab.
const sideWidth = ref(300)
const sideResize = makeSideResizer(sideWidth, 220, 560)

// With no captured requests at all, the browser view drops the list entirely
// and shows the onboarding full-width — the design's "одно пустое состояние".
const browserEmpty = computed(
  () => store.activeView === 'browser' && !store.requests.some((r) => r.source === 'browser')
)

onMounted(() => {
  window.addEventListener('mousemove', sideResize.move)
  window.addEventListener('mouseup', sideResize.stop)
})

onBeforeUnmount(() => {
  window.removeEventListener('mousemove', sideResize.move)
  window.removeEventListener('mouseup', sideResize.stop)
})
</script>

<template>
  <main class="main">
    <div class="side-layout">
      <div v-if="!browserEmpty && store.activeView !== 'collections'" class="side-panel" :style="{ width: sideWidth + 'px' }">
        <HistoryPanel :source="store.activeView === 'request' ? 'manual' : 'browser'" />
      </div>
      <div v-if="!browserEmpty && store.activeView !== 'collections'" class="resize-handle" @mousedown.prevent="sideResize.start"></div>
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
        <template v-else-if="store.activeView === 'browser'">
          <CaptureBar />
          <ResponseViewer v-if="store.browserSelected" :record="store.browserSelected" />
          <BrowserEmptyState v-else />
        </template>
        <template v-else>
          <div class="empty">
            <span class="empty-title">Коллекции скоро</span>
            <span>Здесь будут сохранённые запросы, сгруппированные в коллекции.</span>
          </div>
        </template>
      </div>
    </div>
  </main>
</template>

<style scoped>
@reference "../../style.css";

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
