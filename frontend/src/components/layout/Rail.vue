<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { App as Backend } from '../../../bindings/json-inspector'
import { useEnvironmentsStore } from '../../stores/environments'
import { useRequestsStore } from '../../stores/requests'
import { buildSampleRecord } from '../../lib/sample'
import { usePlatform } from '../../composables/usePlatform'
import Icon from '../ui/Icon.vue'
import { IconButton } from '../ui/button'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from '../ui/dropdown-menu'

const store = useRequestsStore()
const envStore = useEnvironmentsStore()
const { customTitlebar, shortcut } = usePlatform()

const envSheetHint = computed(() => shortcut('E'))

const RAIL_ITEMS = [
  { view: 'request', icon: 'arrow-up-right', label: 'Запрос' },
  { view: 'browser', icon: 'record', label: 'Браузер' },
  { view: 'collections', icon: 'folder', label: 'Коллекции' },
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

function loadSample() {
  store.activeView = 'request'
  store.addRequest(buildSampleRecord())
}

function openAbout() {
  Backend.ShowAbout()
}

function requestUpdateCheck() {
  Backend.RequestUpdateCheck()
}

function selectSource(view: RailView) {
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
          <IconButton variant="bare" size="lg" title="Меню">
            <Icon name="menu" :size="17" :stroke-width="1.6" />
          </IconButton>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <DropdownMenuItem @select="loadSample">
            <Icon name="sparkles" :size="14" /> Загрузить образец
          </DropdownMenuItem>
          <DropdownMenuItem @select="requestUpdateCheck">
            <Icon name="arrow-down" :size="14" /> Проверить обновления
          </DropdownMenuItem>
          <!-- On macOS this lives in the native app menu instead. -->
          <DropdownMenuItem v-if="customTitlebar" @select="openAbout">
            <Icon name="info" :size="14" /> О программе
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
      <span class="rail-label">{{ item.label }}</span>
      <span v-if="item.view === 'browser' && store.unreadCount > 0" class="rail-badge">
        {{ store.unreadCount }}
      </span>
    </button>

    <div class="rail-spacer"></div>

    <IconButton variant="subtle" size="lg" :hint="`Переменные окружения (${envSheetHint})`" @click="envStore.openSheet()">
      <Icon name="settings-2" :size="16" :stroke-width="1.6" />
    </IconButton>
  </aside>
</template>
