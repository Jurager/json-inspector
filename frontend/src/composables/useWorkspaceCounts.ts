import { useWorkspacesStore } from '../stores/workspaces'
import { useMessages } from '../i18n'
import type { Workspace } from '../../bindings/json-inspector/internal/domain'

// What a workspace holds, said in the three places the window says it: a row of the switcher, a row of
// the manager's rail, and the Contents block of the pane. One function, because the three are one
// reading — and because a row that counted folders while the block did not would be two answers to one
// question.
export function useWorkspaceCounts() {
  const store = useWorkspacesStore()
  const { t } = useMessages()

  // The row's second line. A space with nothing in it says so rather than showing nothing: "0
  // collections · 0 environments" is true and reads like a list that failed to load.
  function metaOf(workspace: Workspace): string {
    const held = store.countsOf(workspace.id)
    const parts: string[] = []
    if (held.collections > 0) parts.push(t('counts.collections', held.collections))
    if (held.environments > 0) parts.push(t('counts.environments', held.environments))
    return parts.length ? parts.join(' · ') : t('workspaces.empty')
  }

  // The Contents block: the figure, and the thing it counts — a word that agrees with the figure in
  // languages that have more than two forms, which is why the label is counted too and not fixed.
  function contentsOf(workspace: Workspace) {
    const held = store.countsOf(workspace.id)
    return [
      { value: held.collections, label: t('workspaces.contents.collections', held.collections) },
      { value: held.environments, label: t('workspaces.contents.environments', held.environments) },
      { value: held.runs, label: t('workspaces.contents.runs', held.runs) },
    ]
  }

  return { metaOf, contentsOf }
}
