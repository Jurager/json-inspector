import { onBeforeUnmount, onMounted } from 'vue'
import { useEnvironmentsStore } from '../stores/environments'
import { useRequestsStore } from '../stores/requests'

const HISTORY_STORAGE_KEY = 'ji-history-v1'
const UI_STORAGE_KEY = 'ji-ui-v1'
const MAX_HISTORY = 200
const SAVE_DEBOUNCE_MS = 300

// History and panel layout outlive a window: read once at startup, written back
// on every change. Secrets deliberately stay out — they live in the keychain.
export function useSessionPersistence(
  store: ReturnType<typeof useRequestsStore>,
  envStore: ReturnType<typeof useEnvironmentsStore>
) {
  let unsubscribe: (() => void) | null = null
  let saveTimer: ReturnType<typeof setTimeout> | null = null

  onMounted(() => {
    try {
      const raw = localStorage.getItem(HISTORY_STORAGE_KEY)
      if (raw) {
        const parsed = JSON.parse(raw)
        if (Array.isArray(parsed)) store.hydrate(parsed)
      }
    } catch {
      // ignore corrupt storage
    }

    try {
      const raw = localStorage.getItem(UI_STORAGE_KEY)
      if (raw) {
        const parsed = JSON.parse(raw)
        if (parsed && typeof parsed.open === 'boolean') store.setInspector({ open: parsed.open })
        if (parsed && typeof parsed.width === 'number') store.setInspector({ width: parsed.width })
      }
    } catch {
      // ignore corrupt storage
    }

    // Secrets come back out of the keychain before the first request needs one.
    // The environments now live in the database: read them, take over what the old build left in
    // localStorage, and give a fresh install the environment it has always started with.
    envStore
      .load()
      .then(() => envStore.importLegacyOnce())
      .then(() => envStore.ensureDefaults())
      .catch(() => {
        // Nothing here is worth blocking the window over: an empty environments list is a state
        // the sheet can show.
      })

    unsubscribe = store.$subscribe((_m, state) => {
      if (saveTimer) clearTimeout(saveTimer)
      saveTimer = setTimeout(() => {
        try {
          localStorage.setItem(HISTORY_STORAGE_KEY, JSON.stringify(state.requests.slice(0, MAX_HISTORY)))
          localStorage.setItem(
            UI_STORAGE_KEY,
            JSON.stringify({ open: state.inspector.open, width: state.inspector.width })
          )
        } catch {
          // ignore quota errors
        }
      }, SAVE_DEBOUNCE_MS)
    })
  })

  onBeforeUnmount(() => {
    if (unsubscribe) unsubscribe()
    if (saveTimer) clearTimeout(saveTimer)
  })
}
