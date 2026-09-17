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
        store.setCaptureState({
          connected: true,
          recording: ev.data.recording,
          paused: ev.data.paused,
          tabs: ev.data.tabs,
          browser: ev.data.browser ?? '',
          tabList: ev.data.tabList ?? [],
        })
        // Every state frame is also the one moment the app knows the extension is listening, and the
        // protocol has no acknowledgement: a frame sent while nothing was connected is a frame that
        // evaporated. So the rules are said again here — a few hundred bytes, a handful of times per
        // session, and the only way an extension that has just reconnected learns what it missed.
        void store.applyCaptureFilters()
      }),
      Events.On('capture-disconnected', () => {
        store.setCaptureState({
          connected: false,
          recording: false,
          paused: false,
          tabs: 0,
          browser: '',
          tabList: [],
        })
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
