import { onBeforeUnmount, onMounted, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import type { Info as UpdateInfo } from '../../bindings/json-inspector/internal/infra/updater'

// What the main window knows about updates: the startup check's event, which the status bar
// turns into its link. Checking and installing live in the About window — this link opens it.
const availableUpdate = ref<UpdateInfo | null>(null)

let off: (() => void) | null = null

export function useUpdates() {
  onMounted(() => {
    if (off) return
    off = Events.On('update-available', (ev) => {
      availableUpdate.value = ev.data as UpdateInfo
    })
  })

  onBeforeUnmount(() => {
    off?.()
    off = null
  })

  return { availableUpdate }
}
