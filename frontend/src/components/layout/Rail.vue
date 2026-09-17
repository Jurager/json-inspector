<script setup lang="ts">
import {
  RecordsService,
  SystemService,
  UpdateService,
} from '../../../bindings/json-inspector/internal/transport/wails'
import { useEnvironmentsStore } from '../../stores/environments'
import { useRequestsStore } from '../../stores/requests'
import { useCollectionsStore } from '../../stores/collections'
import { buildSampleRecord } from '../../lib/sample'
import { useMessages } from '../../i18n'
import { usePlatform } from '../../composables/usePlatform'
import Icon from '../ui/Icon.vue'
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

async function loadSample() {
  const record = await RecordsService.Ingest(buildSampleRecord())
  // The event that announces it may not have reached the window yet, and the pane cannot select a
  // record the mirror does not hold.
  store.prepend(record)
  store.activeView = 'request'
  // A sample is meant to be played with, so it opens in the request as well as in the pane.
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

// The rail's update badge asks the About window to check, which is where the answer is drawn: it is
// a window of its own, and this one cannot show the result.
function requestUpdateCheck() {
  UpdateService.RequestCheck()
}

async function selectSource(view: RailView) {
  // A card with unsaved edits is not left quietly, whichever way the user leaves it.
  if (view !== 'collections' && !(await collections.askUnsaved())) return
  if (view === 'browser') store.clearUnreadCaptures()
  store.activeView = view
}
</script>

<template>
  <aside class="sidebar">
    <!-- The hamburger opens the rail, the gear closes it. Neither sits in a strip with a hairline any
         more: the rail is one column of buttons now, and the lines it used to carry across the window
         are gone with them. -->
    <div class="rail-head">
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <button class="rail-btn" :title="t('rail.menu')">
            <Icon name="menu" :size="21" :stroke-width="1.8" />
          </button>
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

    <div class="rail-body">
      <button
        v-for="item in RAIL_ITEMS"
        :key="item.view"
        class="rail-item"
        :class="{ active: store.activeView === item.view }"
        @click="selectSource(item.view)"
      >
        <span class="rail-icon"><Icon :name="item.icon" :size="23" :stroke-width="1.7" /></span>
        <span class="rail-label">{{ t(item.label) }}</span>
        <span v-if="item.view === 'browser' && store.unreadCount > 0" class="rail-badge">
          {{ store.unreadCount }}
        </span>
      </button>

      <div class="rail-spacer"></div>
    </div>

    <!-- Settings has a footer of its own: it is not the last thing in the rail's list of sources but
         the door out of it. -->
    <div class="rail-footer">
      <button class="rail-btn" :title="t('rail.settings')" @click="openSettings()">
        <Icon name="settings-2" :size="21" :stroke-width="1.8" />
      </button>
    </div>
  </aside>
</template>
