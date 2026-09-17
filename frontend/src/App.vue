<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useEnvironmentsStore } from './stores/environments'
import { useRequestsStore } from './stores/requests'
import { useCollectionsStore } from './stores/collections'
import { useSearchStore } from './stores/search'
import { useWorkspacesStore } from './stores/workspaces'
import { useCaptureEvents } from './composables/useCaptureEvents'
import { useRecordEvents } from './composables/useRecordEvents'
import { useGlobalShortcuts } from './composables/useGlobalShortcuts'
import { useCollectionKeys } from './composables/useCollectionKeys'
import { useSessionPersistence } from './composables/useSessionPersistence'
import { useWorkspaceEvents } from './composables/useWorkspaceEvents'
import { useWorkspaceTint } from './composables/useWorkspaceTint'
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
import SearchPalette from './components/search/SearchPalette.vue'
import WorkspacesSheet from './components/workspaces/WorkspacesSheet.vue'
import UnsavedChangesDialog from './components/collections/UnsavedChangesDialog.vue'
import UnsavedLineDialog from './components/request/UnsavedLineDialog.vue'
import SignInModal from './components/settings/SignInModal.vue'
import SignOutSheet from './components/settings/SignOutSheet.vue'
import { useAccount } from './composables/useAccount'
import Toast from './components/ui/Toast.vue'
import { Button } from './components/ui/button'

// The shell: layout plus the composables that own app-wide behaviour. State reaches
// components through the store or a composable, never through props from here.
const { t, te } = useMessages()

const store = useRequestsStore()
const collections = useCollectionsStore()
const envStore = useEnvironmentsStore()
const search = useSearchStore()
const workspaces = useWorkspacesStore()
const { availableUpdate } = useUpdates()
const {
  state: accountState,
  signInOpen,
  signOutOpen,
  beginSignIn,
  cancelSignIn,
  closeSignIn,
  closeSignOut,
  signOut,
} = useAccount()
const account = computed(() => accountState.value?.account ?? null)

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

// The folder's refusal is not the database's with a different title: there is no database file
// behind it yet. So the sentence about one surviving would be a lie, the path below would name a
// file that is not there, and the folder the other button opens is the one that could not be made.
// What is left is the title, the system's own reason, and a button that can be pressed again —
// which is the whole point of that screen, since a locked or redirected folder is usually temporary.
// The kind is Go's FailureDataDir, and it reaches the window as a plain string: it names a message
// key above, so it could not be an enum and still be worded here.
const folderRefused = computed(() => startup.value?.failure?.kind === 'data-dir')

onMounted(loadStartup)

// The status bar's "Доступна версия X" is a pointer to the window that can act on it.
function openAbout() {
  SystemService.ShowAbout()
}

useSessionPersistence(store, collections, envStore)
useWorkspaceEvents()
useWorkspaceTint()
useCaptureEvents(store)
useRecordEvents(store, collections)
useGlobalShortcuts(envStore)
// The tree's own keys live apart from the window's: they act on the collections panel, and
// only while it is the view on screen.
useCollectionKeys()

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
      <p v-if="!folderRefused" class="startup-hint">{{ t('startup.hint') }}</p>
      <pre v-if="startup.failure?.detail" class="startup-detail">{{ startup.failure.detail }}</pre>
      <div class="startup-actions">
        <Button variant="primary" :disabled="retrying" @click="retryInit">
          {{ retrying ? t('startup.retrying') : t('startup.retry') }}
        </Button>
        <Button v-if="!folderRefused" variant="outline" @click="SystemService.OpenDataFolder()">
          {{ t('startup.openDataFolder') }}
        </Button>
      </div>
      <div v-if="!folderRefused" class="startup-path">{{ startup.dbPath }}</div>
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

  <!-- The palette is mounted for the window's whole life and opens by its own flag: a dialog that
       comes and goes with a `v-if` loses the animation that closes it. -->
  <SearchPalette v-if="startup?.ready" :open="search.open" @close="search.close()" />

  <!-- The manager window, in the same frame as the environments one: the spaces on the left, the
       form or the space itself on the right. -->
  <WorkspacesSheet v-if="startup?.ready && workspaces.sheetOpen" @close="workspaces.closeSheet()" />

  <!-- The account's two dialogs hang off the window root and not off the rail: the rail draws a
       material, and a fixed overlay inside it would cover the rail instead of the window. -->
  <SignInModal
    v-if="startup?.ready && signInOpen"
    :server="account?.server ?? ''"
    @begin="beginSignIn"
    @cancel="cancelSignIn"
    @close="closeSignIn()"
  />
  <SignOutSheet v-if="startup?.ready && signOutOpen" @close="closeSignOut()" @confirm="signOut()" />

  <!-- One alert for the whole window: what asks to leave a card with unsaved edits is not always
       the same view. -->
  <UnsavedChangesDialog v-if="startup?.ready" />
  <!-- And its twin for the command line, which asks before a record from history replaces it. -->
  <UnsavedLineDialog v-if="startup?.ready" />

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
