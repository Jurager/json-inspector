<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { RecordsService, SystemService } from '../../../bindings/json-inspector/internal/transport/wails'
import { useEnvironmentsStore } from '../../stores/environments'
import { useRequestsStore } from '../../stores/requests'
import { useCollectionsStore } from '../../stores/collections'
import { buildSampleRecord } from '../../lib/sample'
import { useMessages } from '../../i18n'
import { usePlatform } from '../../composables/usePlatform'
import Icon from '../ui/Icon.vue'
import { IconButton } from '../ui/button'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from '../ui/dropdown-menu'

const { t } = useMessages()

const store = useRequestsStore()
const collections = useCollectionsStore()
const envStore = useEnvironmentsStore()
const { customTitlebar } = usePlatform()

// A tile holds a message key rather than its label: which tile is which does not change with the
// language, and looking the words up as it is drawn is what lets them change without a reload.
const RAIL_ITEMS = [
  { view: 'request', icon: 'arrow-up-right', label: 'rail.request' },
  { view: 'browser', icon: 'record', label: 'rail.browser' },
  { view: 'collections', icon: 'folder', label: 'rail.collections' },
] as const

type RailView = (typeof RAIL_ITEMS)[number]['view']

const railEl = ref<HTMLElement | null>(null)
const tiles = ref<HTMLElement[]>([])

const activeIndex = computed(() => RAIL_ITEMS.findIndex((item) => item.view === store.activeView))

const MARKER_INSET = 5

const marker = reactive({ y: 0, height: 0, visible: false })

function syncMarker() {
  const tile = tiles.value[activeIndex.value]
  if (!tile) return
  marker.y = tile.offsetTop + MARKER_INSET
  marker.height = tile.offsetHeight - MARKER_INSET * 2
  marker.visible = true
}

let railObserver: ResizeObserver | undefined

onMounted(() => {
  syncMarker()
  if (railEl.value) {
    railObserver = new ResizeObserver(syncMarker)
    railObserver.observe(railEl.value)
  }
})

onBeforeUnmount(() => railObserver?.disconnect())

watch(activeIndex, syncMarker, { flush: 'post' })

async function loadSample() {
  const record = await RecordsService.Ingest(buildSampleRecord())
  // The event that announces it may not have reached the window yet, and the pane cannot select a
  // record the mirror does not hold.
  store.prepend(record)
  store.activeView = 'request'
  await store.selectManual(record.id)
}

function openAbout() {
  SystemService.ShowAbout()
}

// The gear is the settings window, as the handoff has it. The environments sheet keeps its two other
// doors — the title bar's dropdown and its own shortcut — so nothing lost a way in.
function openSettings() {
  SystemService.ShowSettings()
}

function requestUpdateCheck() {
  SystemService.RequestUpdateCheck()
}

async function selectSource(view: RailView) {
  // A card with unsaved edits is not left quietly, whichever way the user leaves it.
  if (view !== 'collections' && !(await collections.askUnsaved())) return
  if (view === 'browser') store.clearUnreadCaptures()
  store.activeView = view
}
</script>

<template>
  <aside ref="railEl" class="sidebar">
    <span
      v-if="marker.visible"
      class="rail-marker"
      :style="{ transform: `translateY(${marker.y}px)`, height: `${marker.height}px` }"
    ></span>

    <div class="rail-menu-wrap">
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <IconButton variant="bare" size="lg" :title="t('rail.menu')">
            <Icon name="menu" :size="17" :stroke-width="1.6" />
          </IconButton>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <DropdownMenuItem @select="loadSample">
            <Icon name="sparkles" :size="14" /> {{ t('rail.loadSample') }}
          </DropdownMenuItem>
          <DropdownMenuItem @select="requestUpdateCheck">
            <Icon name="arrow-down" :size="14" /> {{ t('rail.checkUpdates') }}
          </DropdownMenuItem>
          <!-- On macOS this lives in the native app menu instead. -->
          <DropdownMenuItem v-if="customTitlebar" @select="openAbout">
            <Icon name="info" :size="14" /> {{ t('rail.about') }}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>

    <div class="rail-divider"></div>

    <button
      v-for="item in RAIL_ITEMS"
      :key="item.view"
      ref="tiles"
      class="rail-item"
      :class="{ active: store.activeView === item.view }"
      @click="selectSource(item.view)"
    >
      <span class="rail-icon"><Icon :name="item.icon" :size="18" /></span>
      <span class="rail-label">{{ t(item.label) }}</span>
      <span v-if="item.view === 'browser' && store.unreadCount > 0" class="rail-badge">
        {{ store.unreadCount }}
      </span>
    </button>

    <div class="rail-spacer"></div>

    <IconButton
      variant="subtle"
      size="lg"
      :hint="t('rail.settings')"
      @click="openSettings()"
    >
      <Icon name="settings-2" :size="16" :stroke-width="1.6" />
    </IconButton>
  </aside>
</template>
