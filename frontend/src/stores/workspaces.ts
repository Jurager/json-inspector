import { defineStore } from 'pinia'
import { WorkspaceService } from '../../bindings/json-inspector/internal/transport/wails'
import type {
  Workspace,
  WorkspaceCounts,
  WorkspaceState,
} from '../../bindings/json-inspector/internal/domain'
import type { CreateInput, Patch } from '../../bindings/json-inspector/internal/usecase/workspace'
import { t as tr } from '../i18n'

// The workspaces the app keeps things in, and which one it is showing. Vue decides nothing here: the
// list, the names, the counts and the choice all come from Go, and a call answers with the whole
// state — the switcher draws all of it at once, so a partial answer would only be a second thing to
// keep.
export const useWorkspacesStore = defineStore('workspaces', {
  state: () => ({
    state: null as WorkspaceState | null,
    // The manager window: whether it is up, which space it is about, and which pane the right-hand
    // side is showing. It is window state and not data, but the switcher's own button raises it too,
    // so it cannot live inside the window's component the way the environments sheet's does.
    sheetOpen: false,
    creating: false,
    editedId: null as string | null,
  }),

  getters: {
    list: (s): Workspace[] => s.state?.workspaces ?? [],
    activeId: (s): string => s.state?.activeId ?? '',
    active(): Workspace | null {
      return this.list.find((w) => w.id === this.activeId) ?? null
    },
    // The window is about one space: the row it was opened on, or the one on screen when it was the
    // switcher that opened it.
    edited(): Workspace | null {
      const id = this.editedId ?? this.activeId
      return this.list.find((w) => w.id === id) ?? null
    },
    // The wire marks the map as possibly null, and a value may be absent: a space with nothing in it
    // has no entry at all. Both are settled in countsOf, once, rather than at every use.
    counts(s): Record<string, WorkspaceCounts | undefined> {
      return s.state?.counts ?? {}
    },
  },

  actions: {
    async load() {
      this.state = await WorkspaceService.Snapshot()
    },

    // A switch is one call to Go and nothing else here: the caller reloads the data below the window
    // (history, collections, environments, the draft), because only it knows what is drawn.
    async switch(id: string) {
      this.state = await WorkspaceService.Switch(id)
    },

    // The id of what was made is worked out from the two lists rather than read off `activeId`: a new
    // space is not the one on screen, and the window that just made it wants to show it.
    async create(input: CreateInput): Promise<string> {
      const before = new Set(this.list.map((w) => w.id))
      this.state = await WorkspaceService.Create(input)
      const made = this.list.find((w) => !before.has(w.id))
      this.editedId = made?.id ?? null
      this.creating = false
      return made?.id ?? ''
    },

    async update(id: string, patch: Patch) {
      this.state = await WorkspaceService.Update(id, patch)
    },

    async remove(id: string) {
      this.state = await WorkspaceService.Delete(id)
    },

    // What a space holds, for the rows that draw it. A space the map does not name holds nothing: the
    // store's own answer for it is a row of zeros rather than a missing line.
    countsOf(id: string): WorkspaceCounts {
      return this.counts[id] ?? { collections: 0, environments: 0, runs: 0 }
    },

    // ---- the window ------------------------------------------------------

    // Opened on a row — the switcher's Manage, or a row in the rail — or on the space on screen when
    // nobody named one.
    openSheet(id: string | null = null) {
      this.editedId = id ?? this.activeId
      this.creating = false
      this.sheetOpen = true
    },

    // The create form, from either "New workspace" button.
    openCreate() {
      this.creating = true
      this.editedId = this.activeId
      this.sheetOpen = true
    },

    // A row in the rail: which space the window is about, with the form put away.
    edit(id: string) {
      this.editedId = id
      this.creating = false
    },

    cancelCreating() {
      this.creating = false
    },

    closeSheet() {
      this.sheetOpen = false
      this.creating = false
    },
  },
})

// displayName is what the switcher shows: the name the user gave it, or the interface's own word for
// the space the app starts with. That one is stored with an empty name on purpose — "Личное" is a
// word of whatever language the window is in, so it cannot be frozen into the row.
export function workspaceName(workspace: Workspace): string {
  return workspace.name || tr('workspaces.personal')
}

// workspaceInitial is the letter on the avatar, the way the mockup draws it: the first letter of
// whatever name is shown, which for the default workspace is the letter of the translated word.
export function workspaceInitial(workspace: Workspace): string {
  return workspaceName(workspace).trim().charAt(0).toUpperCase() || '·'
}
