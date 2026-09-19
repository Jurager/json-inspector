import { defineStore } from 'pinia'
import type { VarResolution } from '../lib/vars'
import { EnvironmentsService } from '../../bindings/json-inspector/internal/transport/wails'
import { VariableKind } from '../../bindings/json-inspector/internal/domain'
import { t as tr } from '../i18n'
import type { EnvScope, EnvState, Environment, Variable } from '../../bindings/json-inspector/internal/domain'
import type { Entry } from '../../bindings/json-inspector/internal/dotenv'
import type { EnvironmentDraft } from '../../bindings/json-inspector/internal/usecase/environment'

function scopeOf(envId: string | null): EnvScope {
  return envId === null ? {} : { environment: envId }
}

// The wire marks a list as possibly null because Go can marshal a nil slice that way; the app never
// means that, so the mirror settles it once here instead of at every use.
export type Env = Omit<Environment, 'vars'> & { vars: Variable[] }

// The mirror: every field comes from Go, and nothing here is derived state that Go also holds. The
// one exception is resolution below, which stays local while the draft it serves is still local —
// it reads the same values and kinds Go does, so the two cannot disagree.
export const useEnvironmentsStore = defineStore('environments', {
  state: () => ({
    envState: null as EnvState | null,
    sheetOpen: false,
    sheetFocus: null as { envId: string | null; varName: string } | null,
    editedEnvId: null as string | null,
    // Values the user asked to see, by variable id. Filled by reveal() alone, dropped when the
    // sheet closes: a secret is not carried around just because it exists.
    revealed: {} as Record<string, string>,
  }),

  getters: {
    environments(state): Env[] {
      return (state.envState?.environments ?? []).map((e) => ({ ...e, vars: e.vars ?? [] }))
    },

    globals(state): Variable[] {
      return state.envState?.globals ?? []
    },

    activeId(state): string | null {
      return state.envState?.activeId || null
    },

    activeEnvironment(): Env | null {
      return this.environments.find((e) => e.id === this.activeId) ?? null
    },
  },

  actions: {
    // ---- state from Go ---------------------------------------------------

    async load() {
      this.envState = await EnvironmentsService.Snapshot()
    },

    // What a workspace switch leaves behind: every environment on screen belongs to the space being
    // left. A revealed secret is dropped for the same reason — it is a value of a variable that is
    // no longer there.
    forget() {
      this.envState = null
      this.revealed = {}
      this.editedEnvId = null
      this.sheetFocus = null
    },


    // A fresh database has nowhere to type a base URL, which is a poor first screen.
    async ensureDefaults() {
      if (this.environments.length > 0 || this.globals.length > 0) return
      this.envState = await EnvironmentsService.EnsureDefaults()
    },

    // ---- environments ----------------------------------------------------

    async setActive(id: string | null) {
      this.envState = await EnvironmentsService.ActivateEnvironment(id ?? '')
    },

    // The sheet's create form in one call: a name, a colour and what it starts from. It answers with
    // the id of what it made — worked out from the two lists rather than read off `activeId`, which
    // happens to hold it only because Create activates what it writes.
    async createEnv(draft: EnvironmentDraft): Promise<string> {
      const before = new Set(this.environments.map((e) => e.id))
      this.envState = await EnvironmentsService.CreateEnvironment(draft)
      const made = this.environments.find((e) => !before.has(e.id))
      this.editedEnvId = made?.id ?? null
      this.sheetFocus = null
      return made?.id ?? ''
    },

    async removeEnv(id: string) {
      this.envState = await EnvironmentsService.DeleteEnvironment(id)
      this.forgetRevealed(this.environments)
    },

    // Values revealed for a variable that no longer exists go with it. `was` is the state before the
    // call, because what is being dropped is what the call took away.
    forgetRevealed(was: Env[]) {
      const mine = new Set<string>()
      for (const env of was) for (const v of env.vars) mine.add(v.id)
      for (const v of this.globals) mine.add(v.id)

      const live = new Set<string>()
      for (const env of this.environments) for (const v of env.vars) live.add(v.id)
      for (const v of this.globals) live.add(v.id)

      for (const key of Object.keys(this.revealed)) {
        if (mine.has(key) && !live.has(key)) delete this.revealed[key]
      }
    },

    async renameEnv(id: string, name: string) {
      this.envState = await EnvironmentsService.UpdateEnvironment(id, { name })
    },

    async setEnvColor(id: string, color: string) {
      this.envState = await EnvironmentsService.UpdateEnvironment(id, { color })
    },

    // The Access switch. Unlocking a read-only environment is this write and nothing else: the flag
    // is what the sheet reads to decide whether the cells are editable, and a session-only unlock
    // beside it would be a second answer to the same question.
    async setEnvReadonly(id: string, readonly: boolean) {
      this.envState = await EnvironmentsService.UpdateEnvironment(id, { readonly })
    },

    // The globals cannot be deleted, only emptied; what comes back is the state without them.
    async clearGlobals() {
      const before = this.environments
      this.envState = await EnvironmentsService.ClearGlobals()
      this.forgetRevealed(before)
    },

    // ---- variables -------------------------------------------------------

    // The sheet's "add row" leaves this empty and starts editing it; an import writes values of its
    // own. The new row lands last in its scope, which is what addVar's caller gets back.
    async addVar(envId: string | null, init: Partial<Variable> = {}): Promise<string> {
      this.envState = await EnvironmentsService.AddVariable(scopeOf(envId), {
        name: init.name ?? '',
        kind: init.kind ?? VariableKind.VariableText,
        value: init.value ?? '',
      })
      return this.varsOf(envId).at(-1)?.id ?? ''
    },

    // A value that is absent means "leave the stored one alone": the sheet never has a secret's
    // value to send back, so renaming one must not blank it.
    async updateVar(envId: string | null, varId: string, patch: Partial<Variable>) {
      const current = this.varsOf(envId).find((v) => v.id === varId)
      if (!current) return
      this.envState = await EnvironmentsService.UpdateVariable(scopeOf(envId), {
        id: varId,
        name: patch.name ?? current.name,
        kind: patch.kind ?? current.kind,
        enabled: patch.enabled ?? current.enabled,
        setValue: patch.value !== undefined,
        value: patch.value ?? '',
      })
      // A value the user just typed is the one they should see, revealed or not.
      if (patch.value !== undefined && (patch.kind ?? current.kind) === VariableKind.VariableSecret) {
        this.revealed[varId] = patch.value
      }
    },

    async removeVar(envId: string | null, varId: string) {
      this.envState = await EnvironmentsService.RemoveVariable(scopeOf(envId), varId)
      delete this.revealed[varId]
    },

    // What the file held, as Go parsed it: an existing name is replaced and the rest are kept, which
    // is the rule the import sheet states. The decisions are Go's the moment the file is read, so
    // there is nothing per key for the window to decide.
    async importDotenv(envId: string | null, entries: Entry[]) {
      const kept = entries.filter((e) => e.name.trim() !== '')
      if (kept.length === 0) return
      this.envState = await EnvironmentsService.ImportEntries(scopeOf(envId), kept)
    },

    // ---- secrets ---------------------------------------------------------

    // The eye button: the only path from a stored secret to the window.
    async reveal(varId: string): Promise<string> {
      const value = await EnvironmentsService.Reveal(varId)
      this.revealed[varId] = value
      return value
    },

    hide(varId: string) {
      delete this.revealed[varId]
    },

    isRevealed(varId: string): boolean {
      return this.revealed[varId] !== undefined
    },

    // ---- resolution over the mirror --------------------------------------
    // What is left of it is what the window draws: the tooltip over a `{{token}}` pill, and the
    // export templates. Filling a request in is Go's now, values and all.

    effectiveValue(v: Variable): string {
      if (v.kind !== VariableKind.VariableSecret) return v.value ?? ''
      return this.revealed[v.id] ?? ''
    },

    // envId is the environment a request pinned itself to, when it has one. It answers in place of
    // the window's own, the way the same id does on the Go side — an id nothing answers to is no
    // environment at all there, and it is one here too, rather than quietly the window's.
    resolveVariable(name: string, envId?: string): VarResolution | null {
      const env = envId ? (this.environments.find((e) => e.id === envId) ?? null) : this.activeEnvironment
      if (env) {
        const v = env.vars.find((x) => x.name === name && x.enabled)
        if (v) return { value: this.effectiveValue(v), source: 'env', kind: v.kind }
      }
      const g = this.globals.find((x) => x.name === name && x.enabled)
      if (g) return { value: this.effectiveValue(g), source: 'global', kind: g.kind }
      return null
    },

    rowsFor(envId: string | null): { own: Variable[]; inherited: (Variable & { overridden: boolean })[] } {
      const own = this.varsOf(envId)
      // Edited in the "Глобальные" scope itself, so nothing is inherited there.
      if (envId === null) return { own, inherited: [] }

      const ownNames = new Set(own.map((v) => v.name))
      return {
        own,
        inherited: this.globals.map((g) => ({ ...g, overridden: ownNames.has(g.name) })),
      }
    },

    varsOf(envId: string | null): Variable[] {
      if (envId === null) return this.globals
      return this.environments.find((e) => e.id === envId)?.vars ?? []
    },

    envById(envId: string | null): Env | null {
      if (envId === null) return null
      return this.environments.find((e) => e.id === envId) ?? null
    },

    // ---- sheet -----------------------------------------------------------

    editEnv(id: string | null) {
      this.editedEnvId = id
    },

    openSheet(focus: { envId: string | null; varName: string } | null = null) {
      this.sheetFocus = focus
      this.editedEnvId = focus ? focus.envId : this.activeId
      this.sheetOpen = true
    },

    closeSheet() {
      this.sheetOpen = false
      this.sheetFocus = null
      // Everything the user revealed was revealed for that visit.
      this.revealed = {}
    },

    // What a `{{token}}` in the window asks the sheet to open on. The panel reads it once it has
    // drawn the scope it names, and says so by clearing it.
    takeFocus(): { envId: string | null; varName: string } | null {
      const focus = this.sheetFocus
      if (focus) this.sheetFocus = null
      return focus
    },

  },
})
