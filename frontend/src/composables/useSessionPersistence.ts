import { onBeforeUnmount, onMounted } from 'vue'
import { useEnvironmentsStore } from '../stores/environments'
import { useRequestsStore } from '../stores/requests'
import { useCollectionsStore } from '../stores/collections'
import { useSettings } from './useSettings'

const SAVE_DEBOUNCE_MS = 300

// What outlives a window. The history, the draft being composed and the panel geometry all live in
// the database, so this asks for them on the way in and hands back what the window was still typing
// on the way out.
export function useSessionPersistence(
  store: ReturnType<typeof useRequestsStore>,
  collections: ReturnType<typeof useCollectionsStore>,
  envStore: ReturnType<typeof useEnvironmentsStore>
) {
  const { settings, loadSettings, setLayout } = useSettings()
  let unsubscribe: (() => void) | null = null
  let saveTimer: ReturnType<typeof setTimeout> | null = null

  // A window that is hidden or loses focus is a window the user has stopped typing in: whatever is
  // still in its buffers goes over now, before anything can close it.
  // Both composers are handed over: a card with a half-typed address is as much the window's text as
  // the command line's is, and a hidden window is a window the user stopped typing in.
  const handOver = () => {
    void store.flush()
    void collections.flush()
  }

  onMounted(() => {
    void loadSettings().then(() => {
      const stored = settings.value
      if (!stored) return
      store.setInspector({ open: stored.inspectorOpen, width: stored.inspectorWidth })
    })

    // History first takes over what the old build left in localStorage, then reads the list — which
    // is what makes an imported record appear in the same pass as a stored one.
    store
      .importLegacyOnce()
      .then(() => store.load())
      .then(() => store.loadDraft())
      .catch(() => {
        // An empty list and an empty command line are states the window can show; the next launch
        // tries again.
      })

    window.addEventListener('blur', handOver)
    document.addEventListener('visibilitychange', handOver)

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

    // Only the geometry is left to save: the history belongs to Go and every change to it is
    // already a call on that side.
    unsubscribe = store.$subscribe((_m, state) => {
      if (saveTimer) clearTimeout(saveTimer)
      saveTimer = setTimeout(() => {
        setLayout({ inspectorOpen: state.inspector.open, inspectorWidth: state.inspector.width })
      }, SAVE_DEBOUNCE_MS)
    })
  })

  onBeforeUnmount(() => {
    handOver()
    window.removeEventListener('blur', handOver)
    document.removeEventListener('visibilitychange', handOver)
    if (unsubscribe) unsubscribe()
    if (saveTimer) clearTimeout(saveTimer)
  })
}
