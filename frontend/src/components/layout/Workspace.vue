<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RecordSource } from '../../../bindings/json-inspector/internal/domain'
import { useRequestsStore } from '../../stores/requests'
import { useCollectionsStore } from '../../stores/collections'
import { useResizableWidth } from '../../composables/useResizableWidth'
import { useSettings } from '../../composables/useSettings'
import HistoryPanel from '../history/HistoryPanel.vue'
import RequestBuilder from '../request/RequestBuilder.vue'
import ResponseViewer from '../response/ResponseViewer.vue'
import CaptureBar from '../browser/CaptureBar.vue'
import BrowserEmptyState from '../browser/BrowserEmptyState.vue'
import CollectionTree from '../collections/CollectionTree.vue'
import CollectionOverview from '../collections/CollectionOverview.vue'
import CollectionsEmptyState from '../collections/CollectionsEmptyState.vue'

const store = useRequestsStore()
const collections = useCollectionsStore()
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
void collections.load()

const browserEmpty = computed(
  () =>
    store.activeView === 'browser' && !store.records.some((r) => r.source === RecordSource.SourceBrowser)
)

// Collections with nothing in them are not a list, so the panel is not drawn: the window shows the
// onboarding on the whole width instead.
const collectionsEmpty = computed(() => collections.tree.length === 0)

const sidePanelShown = computed(() => {
  if (store.activeView === 'collections') return !collectionsEmpty.value
  return !browserEmpty.value
})
</script>

<template>
  <main class="main">
    <div class="side-layout">
      <div v-if="sidePanelShown" class="side-panel" :style="{ width: sideWidth + 'px' }">
        <CollectionTree v-if="store.activeView === 'collections'" />
        <HistoryPanel
          v-else
          :source-kind="store.activeView === 'request' ? RecordSource.SourceManual : RecordSource.SourceBrowser"
        />
      </div>
      <div v-if="sidePanelShown" class="resize-handle" @mousedown.prevent="startSideDrag"></div>
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
          <ResponseViewer v-if="store.browserSelected" :record="store.browserSelected" source="browser" />
          <BrowserEmptyState v-else />
        </template>
        <template v-else>
          <CollectionsEmptyState v-if="collectionsEmpty" />
          <template v-else-if="collections.cardOpen">
            <RequestBuilder source="collection" />
            <ResponseViewer v-if="collections.response" :record="collections.response" source="collection" />
            <div v-else-if="collections.loading" class="empty">
              <span class="spinner spinner-lg"></span>
            </div>
            <div v-else class="empty">
              <span class="empty-title">Отправьте запрос</span>
              <span>Отправка не сохраняет правку — для этого «Сохранить» в статус-баре.</span>
            </div>
          </template>
          <CollectionOverview v-else-if="collections.selectedId" />
          <div v-else class="empty">
            <span class="empty-title">Выберите запрос</span>
            <span>Или папку — тогда можно запустить всё, что в ней.</span>
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
