<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRequestsStore } from '../../stores/requests'
import { useResizableWidth } from '../../composables/useResizableWidth'
import { useSettings } from '../../composables/useSettings'
import HistoryPanel from '../history/HistoryPanel.vue'
import RequestBuilder from '../request/RequestBuilder.vue'
import ResponseViewer from '../response/ResponseViewer.vue'
import CaptureBar from '../browser/CaptureBar.vue'
import BrowserEmptyState from '../browser/BrowserEmptyState.vue'

const store = useRequestsStore()
const { settings, loadSettings, setLayout } = useSettings()

// Where the user last left the list. The window paints its own default first; Go's answer replaces
// it as soon as it arrives, which keeps the panel from jumping on a slow start.
const sideWidth = ref(settings.value?.sideWidth ?? 288)
const { startDrag: startSideDrag } = useResizableWidth(sideWidth, { min: 220, max: 560, side: 'left' })

watch(
  () => settings.value?.sideWidth,
  (stored) => {
    if (stored !== undefined && stored !== sideWidth.value) sideWidth.value = stored
  }
)

// Every drag frame lands here; the write itself is collected and sent when the drag settles.
watch(sideWidth, (width) => setLayout({ sideWidth: width }))

void loadSettings()

const browserEmpty = computed(
  () => store.activeView === 'browser' && !store.requests.some((r) => r.source === 'browser')
)
</script>

<template>
  <main class="main">
    <div class="side-layout">
      <div v-if="!browserEmpty && store.activeView !== 'collections'" class="side-panel" :style="{ width: sideWidth + 'px' }">
        <HistoryPanel :source-kind="store.activeView === 'request' ? 'manual' : 'browser'" />
      </div>
      <div v-if="!browserEmpty && store.activeView !== 'collections'" class="resize-handle" @mousedown.prevent="startSideDrag"></div>
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
