<script setup lang="ts">
import { computed } from 'vue'
import { App as Backend } from '../../../bindings/json-inspector'
import { useEnvironmentsStore } from '../../stores/environments'
import { useRequestsStore } from '../../stores/requests'
import { useUpdates } from '../../composables/useUpdates'
import { buildSampleRecord } from '../../lib/sample'
import { shortcut, useCustomTitlebar } from '../../lib/platform'
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
const { checking, check } = useUpdates()

const envSheetHint = computed(() => shortcut('E'))

function loadSample() {
  store.activeView = 'request'
  store.add(buildSampleRecord())
}

function openAbout() {
  Backend.ShowAbout()
}

function openBrowser() {
  store.activeView = 'browser'
  store.markBrowserRead()
}
</script>

<template>
  <aside class="sidebar">
    <div class="rail-menu-wrap">
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <IconButton variant="bare" size="lg" title="Меню">
            <Icon name="menu" :size="18" />
          </IconButton>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <DropdownMenuItem @select="loadSample">
            <Icon name="sparkles" :size="14" /> Загрузить образец
          </DropdownMenuItem>
          <DropdownMenuItem @select="check">
            <Icon name="arrow-down" :size="14" /> {{ checking ? 'Проверка…' : 'Проверить обновления' }}
          </DropdownMenuItem>
          <!-- On macOS this lives in the native app menu instead. -->
          <DropdownMenuItem v-if="useCustomTitlebar" @select="openAbout">
            <Icon name="info" :size="14" /> О программе
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
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
      <span class="rail-icon"><Icon name="record" :size="18" /></span>
      <span class="rail-label">Браузер</span>
      <span v-if="store.unreadCount > 0" class="rail-badge">{{ store.unreadCount }}</span>
    </button>
    <button
      class="rail-item"
      :class="{ active: store.activeView === 'collections' }"
      @click="store.activeView = 'collections'"
    >
      <span class="rail-icon"><Icon name="folder" :size="18" /></span>
      <span class="rail-label">Коллекции</span>
    </button>

    <div class="rail-spacer"></div>

    <IconButton variant="subtle" size="lg" :hint="`Переменные окружения (${envSheetHint})`" @click="envStore.openSheet()">
      <Icon name="settings-2" :size="17" />
    </IconButton>
  </aside>
</template>
