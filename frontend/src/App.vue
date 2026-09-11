<script setup lang="ts">
import { useEnvironmentsStore } from './stores/environments'
import { useRequestsStore } from './stores/requests'
import { useCaptureEvents } from './composables/useCaptureEvents'
import { useGlobalShortcuts } from './composables/useGlobalShortcuts'
import { useSessionPersistence } from './composables/useSessionPersistence'
import { useUpdates } from './composables/useUpdates'
import TitleBar from './components/layout/TitleBar.vue'
import Rail from './components/layout/Rail.vue'
import Workspace from './components/layout/Workspace.vue'
import StatusBar from './components/layout/StatusBar.vue'
import EnvironmentsSheet from './components/environments/EnvironmentsSheet.vue'
import Toast from './components/ui/Toast.vue'
import UpdateModal from './components/update/UpdateModal.vue'

// The shell — layout, plus the composables that own app-wide behaviour: what
// survives a restart (useSessionPersistence), what the extension sends
// (useCaptureEvents), the global keys (useGlobalShortcuts) and the updater
// (useUpdates, shared with the rail, the status bar and the modal).
//
// Nothing is threaded through here by props: a component that needs state
// takes it from the store or from a composable, so adding a feature to the
// rail or the titlebar does not touch this file.
const store = useRequestsStore()
const envStore = useEnvironmentsStore()
const { update, open } = useUpdates()

useSessionPersistence(store, envStore)
useCaptureEvents(store)
useGlobalShortcuts(store, envStore)

// The sheet is an overlay over the whole window, so the command line is still
// mounted underneath — closing hands the caret straight back to it.
function closeSheet() {
  envStore.closeSheet()
  store.requestFocusUrl()
}
</script>

<template>
  <div class="app">
    <TitleBar />
    <div class="body">
      <Rail />
      <Workspace />
    </div>
    <StatusBar :update="update" @open-update="open" />
  </div>

  <EnvironmentsSheet v-if="envStore.sheetOpen" @close="closeSheet" />

  <Toast />
  <UpdateModal />
</template>
