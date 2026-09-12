import { onBeforeUnmount, onMounted } from 'vue'
import { useEnvironmentsStore } from '../stores/environments'
import { useRequestsStore } from '../stores/requests'

// Window-level keys belonging to no particular view; ⌘K is really "focus the
// command line" — see the store.
export function useGlobalShortcuts(
  store: ReturnType<typeof useRequestsStore>,
  envStore: ReturnType<typeof useEnvironmentsStore>
) {
  function onKeydown(e: KeyboardEvent) {
    if (!(e.metaKey || e.ctrlKey)) return
    const key = e.key.toLowerCase()
    if (key === 'k') {
      e.preventDefault()
      store.focusSearch()
    } else if (key === 'e') {
      e.preventDefault()
      if (!envStore.sheetOpen) envStore.openSheet()
    }
  }

  onMounted(() => window.addEventListener('keydown', onKeydown))
  onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
}
