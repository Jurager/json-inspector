<script setup lang="ts">
import { useEnvironmentsStore } from './stores/environments'
import { useRequestsStore } from './stores/requests'
import { useCaptureEvents } from './composables/useCaptureEvents'
import { useGlobalShortcuts } from './composables/useGlobalShortcuts'
import { useSessionPersistence } from './composables/useSessionPersistence'
import { useUpdates } from './composables/useUpdates'
import { focusUrlField } from './composables/urlFocus'
import { App as Backend } from '../bindings/json-inspector'
import TitleBar from './components/layout/TitleBar.vue'
import Rail from './components/layout/Rail.vue'
import Workspace from './components/layout/Workspace.vue'
import StatusBar from './components/layout/StatusBar.vue'
import EnvironmentsSheet from './components/environments/EnvironmentsSheet.vue'
import Toast from './components/ui/Toast.vue'

// The shell: layout plus the composables that own app-wide behaviour. State reaches
// components through the store or a composable, never through props from here.
const store = useRequestsStore()
const envStore = useEnvironmentsStore()
const { availableUpdate } = useUpdates()

// The status bar's "Доступна версия X" is a pointer to the window that can act on it.
function openAbout() {
  Backend.ShowAbout()
}

useSessionPersistence(store, envStore)
useCaptureEvents(store)
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
    <div class="body">
      <Rail />
      <Workspace />
    </div>
    <StatusBar :update-info="availableUpdate" @open-update="openAbout" />
  </div>

  <EnvironmentsSheet v-if="envStore.sheetOpen" @close="closeSheet" />

  <Toast />
</template>
