import { onBeforeUnmount, onMounted } from 'vue'
import { usePlatform } from './usePlatform'
import { useCollectionsStore } from '../stores/collections'
import { useRequestsStore } from '../stores/requests'

// The keys the collections panel answers to. They act on the row that is selected, and only while the
// panel is the view on screen: a collection's name is worth nothing while a request in the command
// line has the focus.
//
// Every one of them is named beside its item in the tree's menu, and the two have to be the same
// thing — a hint that does nothing is worse than no hint at all. The menu is built from these same
// chords through lib/platform, so they cannot drift apart.
export function useCollectionKeys() {
  const collections = useCollectionsStore()
  const requests = useRequestsStore()
  const { chord } = usePlatform()

  // While somebody is typing, every key belongs to the field: a shortcut that fired mid-word would
  // take the text they are writing with it. The tree's own rename box is an input like any other.
  function typing(target: EventTarget | null): boolean {
    const el = target as HTMLElement | null
    if (!el || !el.tagName) return false
    return el.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(el.tagName)
  }

  function onKeydown(e: KeyboardEvent) {
    if (!(e.metaKey || e.ctrlKey)) return
    if (requests.activeView !== 'collections' || typing(e.target)) return
    if (!collections.selectedId) return

    // The physical key, not the letter: on a Russian layout the key that says N carries «т», and a
    // shortcut is a place on the keyboard rather than a letter of one alphabet.
    switch (e.code) {
      case 'KeyN':
        e.preventDefault()
        // The box opens in the tree, on the level the row would join: a request made by a key is named
        // the way one made by the menu is.
        collections.pendingCreate = collections.selectedLevel
        break
      case 'KeyD':
        e.preventDefault()
        void collections.duplicate(collections.selectedId)
        break
      case 'KeyR':
        // WebView2 answers this one itself, and the menu does not print it where that is so: the run is
        // a chord of the platforms that let the app have it.
        if (!chord('R')) break
        e.preventDefault()
        // A run of the level the row belongs to: the selected level is what runNodeId answers with.
        void collections.run(collections.runNodeId, collections.levelName)
        break
    }
  }

  onMounted(() => window.addEventListener('keydown', onKeydown))
  onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
}
