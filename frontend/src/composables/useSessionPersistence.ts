import { onBeforeUnmount, onMounted } from 'vue'
import { useEnvironmentsStore } from '../stores/environments'
import { useRequestsStore } from '../stores/requests'

const HISTORY_KEY = 'ji-history-v1'
const UI_KEY = 'ji-ui-v1'
const MAX_HISTORY = 200
const SAVE_DEBOUNCE_MS = 300

// History and panel layout outlive a window, so they are written back on every
// change and read once at startup. Secrets are deliberately not here: those
// come back from the keychain, see the environments store.
export function useSessionPersistence(
  store: ReturnType<typeof useRequestsStore>,
  envStore: ReturnType<typeof useEnvironmentsStore>
) {
  let unsubscribe: (() => void) | null = null
  let saveTimer: ReturnType<typeof setTimeout> | null = null

  onMounted(() => {
    // Restore request history from the previous session.
    try {
      const raw = localStorage.getItem(HISTORY_KEY)
      if (raw) {
        const parsed = JSON.parse(raw)
        if (Array.isArray(parsed)) store.hydrate(parsed)
      }
    } catch {
      // ignore corrupt storage
    }

    // Restore the inspector's visibility and width.
    try {
      const raw = localStorage.getItem(UI_KEY)
      if (raw) {
        const parsed = JSON.parse(raw)
        if (parsed && typeof parsed.open === 'boolean') store.setInspector({ open: parsed.open })
        if (parsed && typeof parsed.width === 'number') store.setInspector({ width: parsed.width })
      }
    } catch {
      // ignore corrupt storage
    }

    // Secrets are kept in the keychain, not in localStorage, so they have to be
    // pulled back into the session before the first request needs one.
    envStore.hydrateSecrets()

    // Persist request history (debounced).
    unsubscribe = store.$subscribe((_m, state) => {
      if (saveTimer) clearTimeout(saveTimer)
      saveTimer = setTimeout(() => {
        try {
          localStorage.setItem(HISTORY_KEY, JSON.stringify(state.requests.slice(0, MAX_HISTORY)))
          localStorage.setItem(
            UI_KEY,
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
