import { onBeforeUnmount, onMounted, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import type { Info as UpdateInfo } from '../../bindings/json-inspector/internal/usecase/update'

// What the main window knows about updates: the state the backend publishes, which the status bar
// turns into its link. Checking and installing live in the About and update windows — this link
// opens the first of them.
//
// The event carries the whole state rather than only good news, because it also arrives when a
// version is skipped: that is the moment this link has to go away, and nothing else would tell it.
const availableUpdate = ref<UpdateInfo | null>(null)

let off: (() => void) | null = null

export function useUpdates() {
  onMounted(() => {
    if (off) return
    off = Events.On('update-changed', (ev) => {
      const info = ev.data as UpdateInfo
      availableUpdate.value = info.available ? info : null
    })
  })

  onBeforeUnmount(() => {
    off?.()
    off = null
  })

  return { availableUpdate }
}
