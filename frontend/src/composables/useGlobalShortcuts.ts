import { onBeforeUnmount, onMounted } from 'vue'
import { useEnvironmentsStore } from '../stores/environments'
import { useSearchStore } from '../stores/search'

// Window-level keys belonging to no particular view. ⌘K is the palette, which is what the titlebar's
// button promises and what the design gives it; the command line is reached through the palette's own
// "new request" command, so it no longer holds a key of its own.
export function useGlobalShortcuts(envStore: ReturnType<typeof useEnvironmentsStore>) {
  const search = useSearchStore()

  function onKeydown(e: KeyboardEvent) {
    if (!(e.metaKey || e.ctrlKey)) return
    // The physical key, not the letter: on a Russian layout the key that says K carries «л», and a
    // shortcut is a place on the keyboard rather than a letter of one alphabet. The labels beside
    // them in the interface name the key the same way.
    switch (e.code) {
      case 'KeyK':
        e.preventDefault()
        search.openPalette()
        break
      case 'KeyE':
        e.preventDefault()
        if (!envStore.sheetOpen) envStore.openSheet()
        break
    }
  }

  onMounted(() => window.addEventListener('keydown', onKeydown))
  onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
}
