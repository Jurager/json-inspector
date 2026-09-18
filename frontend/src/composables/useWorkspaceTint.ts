import { watch } from 'vue'
import { useWorkspacesStore } from '../stores/workspaces'
import { hasTint } from '../components/workspaces/palette'

// The window's glass takes the colour of the workspace on screen, and no colour is a state of its
// own: a workspace nobody has dressed leaves the window looking exactly as it always has.
//
// The word goes on the document rather than into a style, because what the colour *means* — how much
// of it a wash needs on white glass and how much on nearly black — is the stylesheet's business, and
// it is the same word in both themes. That is also what makes the switch a fade rather than a jump:
// the chrome transitions its own background, and this only says which of the two it is.
export function useWorkspaceTint() {
  const workspaces = useWorkspacesStore()

  watch(
    // What the window wears: what the workspace on screen is. A colour picked in the manager window is
    // written the moment it is picked, so the tint follows the row and not a copy of it.
    () => workspaces.active?.color ?? '',
    (color) => {
      const root = document.documentElement
      if (hasTint(color)) root.dataset.tint = color
      else delete root.dataset.tint
    },
    { immediate: true }
  )
}
