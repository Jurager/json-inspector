import { onBeforeUnmount, onMounted } from 'vue'
import { Events } from '@wailsio/runtime'
import { useWorkspacesStore } from '../stores/workspaces'

// The workspace on screen, as Go announces it. The window that made the change has already drawn it
// — the call it made answered with the whole state — but the news is a broadcast, and a second window
// (or a future one) follows it rather than being told by whoever asked.
export function useWorkspaceEvents() {
  const offs: (() => void)[] = []

  onMounted(() => {
    offs.push(
      Events.On('workspace:changed', () => {
        // The payload names the workspace; the store reads the whole set again rather than patching
        // one row into it, because a rename and a reorder are the same kind of event.
        void useWorkspacesStore().load()
      })
    )
  })

  onBeforeUnmount(() => offs.forEach((off) => off()))
}
