import { onBeforeUnmount, onMounted } from 'vue'
import { useEnvironmentsStore } from '../stores/environments'
import { useRequestsStore } from '../stores/requests'
import { useSettings } from './useSettings'

const HISTORY_STORAGE_KEY = 'ji-history-v1'
const MAX_HISTORY = 200
const SAVE_DEBOUNCE_MS = 300

// What outlives a window: history (still in localStorage until it moves to the database) and the
// panel geometry (already in the database, through the settings feature).
export function useSessionPersistence(
  store: ReturnType<typeof useRequestsStore>,
  envStore: ReturnType<typeof useEnvironmentsStore>
) {
  const { settings, loadSettings, setLayout } = useSettings()
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

    // The panel geometry comes from the database, so it survives a reinstall; the store is what the
    // window draws from while it is open.
    void loadSettings().then(() => {
      const stored = settings.value
      if (!stored) return
      store.setInspector({ open: stored.inspectorOpen, width: stored.inspectorWidth })
    })

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
        } catch {
          // ignore quota errors
        }
        setLayout({ inspectorOpen: state.inspector.open, inspectorWidth: state.inspector.width })
      }, SAVE_DEBOUNCE_MS)
    })
  })

  onBeforeUnmount(() => {
    if (unsubscribe) unsubscribe()
    if (saveTimer) clearTimeout(saveTimer)
  })
}
