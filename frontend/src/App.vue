<script setup lang="ts">
import { useEnvironmentsStore } from './stores/environments'
import { useRequestsStore } from './stores/requests'
import { useCaptureEvents } from './composables/useCaptureEvents'
import { useGlobalShortcuts } from './composables/useGlobalShortcuts'
import { useSessionPersistence } from './composables/useSessionPersistence'
import { useUpdates } from './composables/useUpdates'
import { requestUrlFocus } from './composables/useUrlFocus'
import TitleBar from './components/layout/TitleBar.vue'
import Rail from './components/layout/Rail.vue'
import Workspace from './components/layout/Workspace.vue'
import StatusBar from './components/layout/StatusBar.vue'
import EnvironmentsSheet from './components/environments/EnvironmentsSheet.vue'
import Toast from './components/ui/Toast.vue'
import UpdateModal from './components/update/UpdateModal.vue'

// The shell: layout plus the composables that own app-wide behaviour. State reaches
// components through the store or a composable, never through props from here.
const store = useRequestsStore()
const envStore = useEnvironmentsStore()
const { update, open } = useUpdates()

useSessionPersistence(store, envStore)
useCaptureEvents(store)
useGlobalShortcuts(store, envStore)

// The sheet overlays the window with the command line still mounted underneath,
// so closing hands the caret back to it.
function closeSheet() {
  envStore.closeSheet()
  requestUrlFocus()
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
