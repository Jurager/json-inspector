import { onBeforeUnmount, onMounted } from 'vue'
import { Events } from '@wailsio/runtime'
import { useRequestsStore } from '../stores/requests'

// What the extension sends over the bridge, as it arrives. Kept out of the
// shell so the browser view and the store can grow without the shell growing
// with them.
interface Captured {
  method: string
  url: string
  requestHeaders?: Record<string, string>
  requestBody?: string
  status: number
  statusText?: string
  responseHeaders?: Record<string, string>
  responseBody?: string
  durationMs?: number
  tabTitle?: string
  tabURL?: string
  tabId?: number
  favIconUrl?: string
}

export function useCaptureEvents(store: ReturnType<typeof useRequestsStore>) {
  const offs: (() => void)[] = []

  onMounted(() => {
    // v3 handlers receive a WailsEvent envelope; the payload is on .data.
    offs.push(
      Events.On('captured-request', (ev) => {
        const c = ev.data as Captured
        const headers = c.responseHeaders ?? {}
        store.addCaptured({
          method: c.method,
          url: c.url,
          requestHeaders: c.requestHeaders ?? {},
          requestBody: c.requestBody ?? '',
          status: c.status,
          statusText: c.statusText ?? '',
          responseHeaders: headers,
          responseBody: c.responseBody ?? '',
          durationMs: c.durationMs ?? 0,
          contentType: headers['content-type'] ?? headers['Content-Type'] ?? '',
          tabTitle: c.tabTitle,
          tabURL: c.tabURL,
          tabId: c.tabId,
          favIconUrl: c.favIconUrl,
        })
      }),
      Events.On('capture-state', (ev) => {
        const s = ev.data as { recording: boolean; tabs: number }
        store.setCaptureState({ connected: true, recording: s.recording, tabs: s.tabs })
      }),
      Events.On('capture-disconnected', () => {
        store.setCaptureState({ connected: false, recording: false, tabs: 0 })
      }),
      // The extension's "open this tab" deep link. Subscribed here rather than
      // in the browser list because the list isn't always mounted, and the link
      // must still switch the rail even when there is nothing captured yet.
      Events.On('open-tab', (ev) => {
        store.focusBrowserTab(ev.data as number)
      })
    )
  })

  onBeforeUnmount(() => offs.forEach((off) => off()))
}
