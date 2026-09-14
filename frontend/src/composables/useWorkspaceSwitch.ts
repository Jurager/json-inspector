import { useCollectionsStore } from '../stores/collections'
import { useEnvironmentsStore } from '../stores/environments'
import { useRequestsStore } from '../stores/requests'
import { useWorkspacesStore } from '../stores/workspaces'

// Moving the window to another workspace, which is one gesture in several parts and in this order.
//
// The parts below the window are the mirror of what Go holds, and none of them is about the space
// being entered — so each is emptied and then filled again. The order matters at both ends: what the
// window is still typing goes over *before* the pointer moves (otherwise it would be written into the
// space the user is leaving for), and the cards are asked about unsaved edits before anything moves
// at all.
export async function switchWorkspace(id: string): Promise<boolean> {
  const workspaces = useWorkspacesStore()
  const requests = useRequestsStore()
  const collections = useCollectionsStore()
  const environments = useEnvironmentsStore()

  if (id === workspaces.activeId) return true

  // A card with unsaved edits is not left quietly — the same alert as leaving the collections rail,
  // because exactly the same thing is lost.
  if (!(await collections.askUnsaved())) return false

  // Everything half-typed belongs to the workspace it was typed in.
  await requests.flush()
  await collections.flush()

  try {
    await workspaces.switch(id)
  } catch {
    // A switch that did not happen leaves the window where it was, with what it had: there is
    // nothing to put back.
    return false
  }

  requests.forget()
  collections.forget()
  environments.forget()

  await Promise.all([
    requests.load(),
    requests.loadDraft(),
    requests.loadScripts(),
    collections.load(),
    environments.load(),
  ])
  // A space nobody has been in has no environment yet, and the app has always started with one.
  await environments.ensureDefaults()
  return true
}
