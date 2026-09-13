import { onBeforeUnmount, onMounted } from 'vue'
import { Events } from '@wailsio/runtime'
import { useRequestsStore } from '../stores/requests'

// The state of the extension link. The requests it captures are records, and arrive with every
// other record — see useRecordEvents.
export function useCaptureEvents(store: ReturnType<typeof useRequestsStore>) {
  const offs: (() => void)[] = []

  onMounted(() => {
    // v3 handlers receive a WailsEvent envelope; the payload is on .data.
    offs.push(
      Events.On('capture-state', (ev) => {
        store.setCaptureState({ connected: true, recording: ev.data.recording, tabs: ev.data.tabs })
      }),
      Events.On('capture-disconnected', () => {
        store.setCaptureState({ connected: false, recording: false, tabs: 0 })
      }),
      // Subscribed here, not in the browser list (which isn't always mounted): the link
      // must switch the rail even when nothing has been captured yet.
      Events.On('open-tab', (ev) => {
        store.focusBrowserTab(ev.data)
      })
    )
  })

  onBeforeUnmount(() => offs.forEach((off) => off()))
}
