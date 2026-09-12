import { onBeforeUnmount, onMounted } from 'vue'
import { Events } from '@wailsio/runtime'
import { useRequestsStore } from '../stores/requests'

// The wire shape the extension sends on `captured-request`; every field but these three
// is optional, and the mapping into a RequestRecord below is explicit on purpose.
interface CapturedRequest {
  method: string
  url: string
  status: number
  requestHeaders?: Record<string, string>
  requestBody?: string
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
        const c = ev.data as CapturedRequest
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
      // Subscribed here, not in the browser list (which isn't always mounted): the link
      // must switch the rail even when nothing has been captured yet.
      Events.On('open-tab', (ev) => {
        store.focusBrowserTab(ev.data as number)
      })
    )
  })

  onBeforeUnmount(() => offs.forEach((off) => off()))
}
