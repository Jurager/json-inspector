import { defineStore } from 'pinia'
import type { RequestRecord } from '../lib/types'

let counter = 0
function nextId(): string {
  counter += 1
  return `req-${Date.now()}-${counter}`
}

// Recording flips back to false after 30s of silence from the extension —
// module-level like `counter` above, since it's bookkeeping for the store's
// own action rather than reactive UI state itself.
let recordingTimer: ReturnType<typeof setTimeout> | null = null
const RECORDING_TIMEOUT_MS = 30_000

// Capture is the status bar's source of truth for the browser extension's
// state. Real signals arrive on step 9; until then the only event is the
// arrival of a captured request.
interface CaptureState {
  connected: boolean
  recording: boolean
  tabs: number
}

export const useRequestsStore = defineStore('requests', {
  state: () => ({
    requests: [] as RequestRecord[],
    // The manual request shown in the "Запрос" view.
    manualId: null as string | null,
    // The captured request selected in the "Браузер" view.
    browserId: null as string | null,
    activeView: 'request' as 'request' | 'browser',
    capturing: false,
    unreadCount: 0,
    loading: false,
    capture: { connected: false, recording: false, tabs: 0 } as CaptureState,
  }),
  getters: {
    manualSelected(state): RequestRecord | null {
      return state.requests.find((r) => r.id === state.manualId) ?? null
    },
    browserSelected(state): RequestRecord | null {
      return state.requests.find((r) => r.id === state.browserId) ?? null
    },
  },
  actions: {
    add(record: Omit<RequestRecord, 'id' | 'startedAt'>) {
      const full: RequestRecord = {
        ...record,
        id: nextId(),
        startedAt: Date.now(),
      }
      this.requests.unshift(full)
      this.manualId = full.id
    },
    addCaptured(record: Omit<RequestRecord, 'id' | 'source' | 'startedAt'>) {
      const full: RequestRecord = {
        ...record,
        id: nextId(),
        startedAt: Date.now(),
        source: 'browser',
      }
      this.requests.unshift(full)
      if (this.activeView !== 'browser') {
        this.unreadCount += 1
      }
    },
    hydrate(requests: RequestRecord[]) {
      this.requests = requests
      this.manualId = null
      this.browserId = null
      this.unreadCount = 0
    },
    selectManual(id: string) {
      this.manualId = id
    },
    selectBrowser(id: string) {
      this.browserId = id
    },
    markBrowserRead() {
      this.unreadCount = 0
    },
    setCaptureState(partial: Partial<CaptureState>) {
      this.capture = { ...this.capture, ...partial }
      // While recording, keep the 30s watchdog alive: each captured request
      // pushes the deadline back, so `recording` only drops after a real
      // silence, not between two requests a second apart.
      if (this.capture.recording) {
        if (recordingTimer) clearTimeout(recordingTimer)
        recordingTimer = setTimeout(() => {
          this.capture = { ...this.capture, recording: false }
        }, RECORDING_TIMEOUT_MS)
      } else if (recordingTimer) {
        clearTimeout(recordingTimer)
        recordingTimer = null
      }
    },
    clear() {
      this.requests = []
      this.manualId = null
      this.browserId = null
      this.unreadCount = 0
    },
    clearRequests(ids: string[]) {
      if (ids.length === 0) return
      const set = new Set(ids)
      this.requests = this.requests.filter((r) => !set.has(r.id))
      if (this.manualId && set.has(this.manualId)) this.manualId = null
      if (this.browserId && set.has(this.browserId)) this.browserId = null
    },
  },
})
