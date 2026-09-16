import { defineStore } from 'pinia'
import { WorkspaceService } from '../../bindings/json-inspector/internal/transport/wails'
import { WorkspaceKind } from '../../bindings/json-inspector/internal/domain'
import type { Workspace, WorkspaceState } from '../../bindings/json-inspector/internal/domain'
import type { CreateInput, Patch } from '../../bindings/json-inspector/internal/usecase/workspace'
import { t as tr } from '../i18n'

// The workspaces the app keeps things in, and which one it is showing. Vue decides nothing here: the
// list, the names and the choice all come from Go, and a call answers with the whole state — the
// switcher draws all of it at once, so a partial answer would only be a second thing to keep.
export const useWorkspacesStore = defineStore('workspaces', {
  state: () => ({
    state: null as WorkspaceState | null,
    // The cards are the window's own business, and so is which one is open.
    createOpen: false,
    settingsOpen: false,
    settingsId: null as string | null,
    // A colour the settings card is trying on: the window wears it while the choice is being made,
    // and goes back to what the workspace actually is when the card closes without saving. Null is
    // "nothing is being tried on", which is not the same as a workspace with no colour.
    preview: null as string | null,
  }),

  getters: {
    list: (s): Workspace[] => s.state?.workspaces ?? [],
    activeId: (s): string => s.state?.activeId ?? '',
    active(): Workspace | null {
      return this.list.find((w) => w.id === this.activeId) ?? null
    },
    // The settings card is about one workspace: the one it was opened on, which is not necessarily
    // the one on screen — «Настроить» in the menu is about the row the user pointed at.
    edited(): Workspace | null {
      const id = this.settingsId ?? this.activeId
      return this.list.find((w) => w.id === id) ?? null
    },
    // A team space is the one thing this build cannot make yet, so the switcher says so instead of
    // offering a form that would produce a personal one under a team's name.
    teams(): Workspace[] {
      return this.list.filter((w) => w.kind === WorkspaceKind.WorkspaceTeam)
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

    async create(input: CreateInput) {
      this.state = await WorkspaceService.Create(input)
    },

    async update(id: string, patch: Patch) {
      this.state = await WorkspaceService.Update(id, patch)
    },

    async remove(id: string) {
      this.state = await WorkspaceService.Delete(id)
    },

    // tryColor and stopTrying are the preview of a colour that has not been saved yet: the window is
    // the place the choice is judged in, so it has to show it before it is made.
    tryColor(color: string | null) {
      this.preview = color
    },

    stopTrying() {
      this.preview = null
    },

    openCreate() {
      this.createOpen = true
    },

    openSettings(id: string) {
      this.settingsId = id
      this.settingsOpen = true
    },

    closeCards() {
      this.createOpen = false
      this.settingsOpen = false
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
