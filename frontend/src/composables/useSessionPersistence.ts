import { onBeforeUnmount, onMounted } from 'vue'
import { useEnvironmentsStore } from '../stores/environments'
import { useRequestsStore } from '../stores/requests'
import { useSettings } from './useSettings'

const SAVE_DEBOUNCE_MS = 300

// What outlives a window. Both the history and the panel geometry now live in the database, so this
// only has to ask for them and, for the geometry, say when it changed.
export function useSessionPersistence(
  store: ReturnType<typeof useRequestsStore>,
  envStore: ReturnType<typeof useEnvironmentsStore>
) {
  const { settings, loadSettings, setLayout } = useSettings()
  let unsubscribe: (() => void) | null = null
  let saveTimer: ReturnType<typeof setTimeout> | null = null

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
      .catch(() => {
        // An empty list is a state the panel can show; the next launch tries again.
      })

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
    if (unsubscribe) unsubscribe()
    if (saveTimer) clearTimeout(saveTimer)
  })
}
