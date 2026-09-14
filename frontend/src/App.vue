<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useEnvironmentsStore } from './stores/environments'
import { useRequestsStore } from './stores/requests'
import { useCollectionsStore } from './stores/collections'
import { useWorkspacesStore } from './stores/workspaces'
import { useCaptureEvents } from './composables/useCaptureEvents'
import { useRecordEvents } from './composables/useRecordEvents'
import { useGlobalShortcuts } from './composables/useGlobalShortcuts'
import { useSessionPersistence } from './composables/useSessionPersistence'
import { useWorkspaceEvents } from './composables/useWorkspaceEvents'
import { useUpdates } from './composables/useUpdates'
import { focusUrlField } from './composables/urlFocus'
import { SystemService } from '../bindings/json-inspector/internal/transport/wails'
import type { StartupStatus } from '../bindings/json-inspector/internal/transport/wails'
import { useMessages } from './i18n'
import TitleBar from './components/layout/TitleBar.vue'
import Rail from './components/layout/Rail.vue'
import Workspace from './components/layout/Workspace.vue'
import StatusBar from './components/layout/StatusBar.vue'
import EnvironmentsSheet from './components/environments/EnvironmentsSheet.vue'
import WorkspaceCreateDialog from './components/workspaces/WorkspaceCreateDialog.vue'
import WorkspaceSettingsDialog from './components/workspaces/WorkspaceSettingsDialog.vue'
import UnsavedChangesDialog from './components/collections/UnsavedChangesDialog.vue'
import Toast from './components/ui/Toast.vue'
import { Button } from './components/ui/button'

// The shell: layout plus the composables that own app-wide behaviour. State reaches
// components through the store or a composable, never through props from here.
const { t, te } = useMessages()

const store = useRequestsStore()
const collections = useCollectionsStore()
const envStore = useEnvironmentsStore()
const workspaces = useWorkspacesStore()
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

// Go says which refusal it was and nothing more; the sentence for each kind is here, where the
// language is. A kind this build has no sentence for — one from a newer version — falls back to the
// general one rather than being printed as its own key.
const startupTitle = computed(() => {
  const kind = startup.value?.failure?.kind
  const key = `startup.failed.${kind}`
  return kind && te(key) ? t(key) : t('startup.title')
})

onMounted(loadStartup)

// The status bar's "Доступна версия X" is a pointer to the window that can act on it.
function openAbout() {
  SystemService.ShowAbout()
}

useSessionPersistence(store, collections, envStore)
useWorkspaceEvents()
useCaptureEvents(store)
useRecordEvents(store, collections)
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
      <div class="startup-title">{{ startupTitle }}</div>
      <p class="startup-hint">{{ t('startup.hint') }}</p>
      <pre v-if="startup.failure?.detail" class="startup-detail">{{ startup.failure.detail }}</pre>
      <div class="startup-actions">
        <Button variant="primary" :disabled="retrying" @click="retryInit">
          {{ retrying ? t('startup.retrying') : t('startup.retry') }}
        </Button>
        <Button variant="outline" @click="SystemService.OpenDataFolder()">
          {{ t('startup.openDataFolder') }}
        </Button>
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

  <!-- The two workspace cards float over the window, as the mockup draws them: same fields, same
       segment and same destructive link as the sheets the app already has. -->
  <WorkspaceCreateDialog
    v-if="startup?.ready"
    :open="workspaces.createOpen"
    @close="workspaces.closeCards()"
  />
  <WorkspaceSettingsDialog
    v-if="startup?.ready"
    :open="workspaces.settingsOpen"
    @close="workspaces.closeCards()"
  />

  <!-- One alert for the whole window: what asks to leave a card with unsaved edits is not always
       the same view. -->
  <UnsavedChangesDialog v-if="startup?.ready" />

  <Toast />
</template>

<style scoped>
@reference "./style.css";

/* The page paints nothing — the window shows the material behind it — so a full-width screen has to
   bring its own ground: on the failure screen there is no chrome and no content to do it. */
.startup-failure {
  @apply flex-1 flex flex-col items-center justify-center gap-3 px-8 min-h-0 bg-bg;
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
