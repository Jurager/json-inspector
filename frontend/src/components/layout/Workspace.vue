<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ListSide, RecordSource } from '../../../bindings/json-inspector/internal/domain'
import { useRequestsStore } from '../../stores/requests'
import { useCollectionsStore } from '../../stores/collections'
import { useResizableWidth } from '../../composables/useResizableWidth'
import { useSettings } from '../../composables/useSettings'
import { useMessages } from '../../i18n'
import Icon from '../ui/Icon.vue'
import HistoryPanel from '../history/HistoryPanel.vue'
import RequestBuilder from '../request/RequestBuilder.vue'
import ResponseViewer from '../response/ResponseViewer.vue'
import BrowserEmptyState from '../browser/BrowserEmptyState.vue'
import CollectionTree from '../collections/CollectionTree.vue'
import CollectionOverview from '../collections/CollectionOverview.vue'
import CollectionsEmptyState from '../collections/CollectionsEmptyState.vue'

const store = useRequestsStore()

const { t } = useMessages()
const collections = useCollectionsStore()
const { settings, loadSettings, setLayout } = useSettings()

// Where the user last left the list. The window paints its own default first; Go's answer replaces
// it as soon as it arrives, which keeps the panel from jumping on a slow start.
const sideWidth = ref(settings.value?.sideWidth ?? 288)

// Which edge the list sits on, and whether it is drawn at all — see sidePanelShown below. The handle
// reads it per drag, so a panel moved to the other edge drags the right way without being rebuilt.
const listSide = computed(() => settings.value?.listSide ?? ListSide.ListSideLeft)
const panelOnRight = computed(() => listSide.value === ListSide.ListSideRight)
const { startDrag: startSideDrag } = useResizableWidth(sideWidth, {
  min: 220,
  max: 560,
  side: () => (panelOnRight.value ? 'right' : 'left'),
})

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

// The panel is drawn only when there is something to list in it and the user has not put it away.
// The two are different answers to different questions: an empty list is not shown whatever the
// side says, and the side is still remembered for the list that is coming.
const sidePanelShown = computed(() => {
  if (listSide.value === ListSide.ListSideHidden) return false
  if (store.activeView === 'collections') return !collectionsEmpty.value
  return !browserEmpty.value
})
</script>

<template>
  <main class="main">
    <div class="side-layout" :class="{ 'panel-right': panelOnRight }">
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
            <Icon name="arrow-up-right" :size="32" :stroke-width="1.6" class="empty-icon" />
            <div class="empty-text">
              <span class="empty-title">{{ t('workspace.sendRequest') }}</span>
              <span class="empty-hint">{{ t('workspace.orLoadSample') }}</span>
            </div>
          </div>
        </template>
        <template v-else-if="store.activeView === 'browser'">
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
              <Icon name="arrow-up-right" :size="32" :stroke-width="1.6" class="empty-icon" />
              <div class="empty-text">
                <span class="empty-title">{{ t('workspace.sendRequest') }}</span>
                <span class="empty-hint">{{ t('workspace.sendDoesNotSave') }}</span>
              </div>
            </div>
          </template>
          <CollectionOverview v-else-if="collections.selectedId" />
          <div v-else class="empty">
            <span class="empty-title">{{ t('workspace.chooseRequest') }}</span>
            <span>{{ t('workspace.orCollection') }}</span>
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
  /* The width is the one the user dragged, and it is a preference rather than a right: a window too
     narrow for it takes the panel down to the floor the handle itself stops at, and gives it back
     when there is room again. Without this the request bar absorbs the whole shortage. */
  @apply min-w-0 border-r border-border;
  flex: 0 1 auto;
  min-width: 220px;
}

.resize-handle {
  @apply flex-none w-[5px] -ml-[5px] relative z-1 bg-transparent;
  cursor: col-resize;
  transition: background 0.15s ease;
}

/* The panel is a sibling of the content, not a wrapper around it, so moving it to the other edge is
   an order change and a border swap — the content is never torn down and built again. The handle
   follows the panel: it rides on the panel's inner edge, overlapping it by its own width. */
.side-layout.panel-right .side-panel {
  @apply border-r-0 border-l;
  order: 2;
}

.side-layout.panel-right .resize-handle {
  order: 1;
  margin-left: 0;
  margin-right: -5px;
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
