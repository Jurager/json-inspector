<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useEnvironmentsStore } from './stores/environments'
import { useRequestsStore } from './stores/requests'
import { useCaptureEvents } from './composables/useCaptureEvents'
import { useRecordEvents } from './composables/useRecordEvents'
import { useGlobalShortcuts } from './composables/useGlobalShortcuts'
import { useSessionPersistence } from './composables/useSessionPersistence'
import { useUpdates } from './composables/useUpdates'
import { focusUrlField } from './composables/urlFocus'
import { SystemService } from '../bindings/json-inspector/internal/transport/wails'
import type { StartupStatus } from '../bindings/json-inspector/internal/transport/wails'
import TitleBar from './components/layout/TitleBar.vue'
import Rail from './components/layout/Rail.vue'
import Workspace from './components/layout/Workspace.vue'
import StatusBar from './components/layout/StatusBar.vue'
import EnvironmentsSheet from './components/environments/EnvironmentsSheet.vue'
import Toast from './components/ui/Toast.vue'
import { Button } from './components/ui/button'

// The shell: layout plus the composables that own app-wide behaviour. State reaches
// components through the store or a composable, never through props from here.
const store = useRequestsStore()
const envStore = useEnvironmentsStore()
const { availableUpdate } = useUpdates()

// Without a database every other call fails, and this is the one screen that can say why instead of
// leaving a window full of empty panels.
const startup = ref<StartupStatus | null>(null)
const retrying = ref(false)

async function loadStartup() {
  startup.value = await SystemService.StartupStatus()
}

async function retryInit() {
  retrying.value = true
  try {
    startup.value = await SystemService.RetryInit()
  } finally {
    retrying.value = false
  }
}

onMounted(loadStartup)

// The status bar's "Доступна версия X" is a pointer to the window that can act on it.
function openAbout() {
  SystemService.ShowAbout()
}

useSessionPersistence(store, envStore)
useCaptureEvents(store)
useRecordEvents(store)
useGlobalShortcuts(store, envStore)

// The sheet overlays the window with the command line still mounted underneath,
// so closing hands the caret back to it.
function closeSheet() {
  envStore.closeSheet()
  focusUrlField()
}
</script>

<template>
  <div class="app">
    <TitleBar />

    <div v-if="startup && !startup.ready" class="startup-failure">
      <div class="startup-title">{{ startup.failure?.message ?? 'Не удалось запустить приложение' }}</div>
      <p class="startup-hint">
        Данные не потеряны: файл базы на месте, приложение просто не смогло его открыть.
      </p>
      <pre v-if="startup.failure?.detail" class="startup-detail">{{ startup.failure.detail }}</pre>
      <div class="startup-actions">
        <Button variant="primary" :disabled="retrying" @click="retryInit">
          {{ retrying ? 'Пробуем снова…' : 'Повторить' }}
        </Button>
        <Button variant="outline" @click="SystemService.OpenDataFolder()">Открыть папку данных</Button>
      </div>
      <div class="startup-path">{{ startup.dbPath }}</div>
    </div>

    <template v-else>
      <div class="body">
        <Rail />
        <Workspace />
      </div>
      <StatusBar :update-info="availableUpdate" @open-update="openAbout" />
    </template>
  </div>

  <EnvironmentsSheet v-if="startup?.ready && envStore.sheetOpen" @close="closeSheet" />

  <Toast />
</template>

<style scoped>
@reference "./style.css";

.startup-failure {
  @apply flex-1 flex flex-col items-center justify-center gap-3 px-8 min-h-0;
}

.startup-title {
  @apply text-[15px] font-semibold text-text;
}

.startup-hint {
  @apply text-[12.5px] text-text-secondary text-center max-w-[420px];
}

/* Machine text, so it gets the mono face and a scroll of its own rather than wrapping the
   window into a wall of text. */
.startup-detail {
  @apply max-w-[520px] max-h-[160px] overflow-auto text-[11.5px] text-text-tertiary;
  font-family: var(--mono);
  background: var(--bg-inset);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 8px 10px;
  white-space: pre-wrap;
}

.startup-actions {
  @apply flex items-center gap-2 mt-1;
}

.startup-path {
  @apply text-[11px] text-text-tertiary;
  font-family: var(--mono);
}
</style>
