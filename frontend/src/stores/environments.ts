import { defineStore } from 'pinia'
import type { VarResolution } from '../lib/vars'
import { EnvironmentsService } from '../../bindings/json-inspector/internal/transport/wails'
import { VariableKind } from '../../bindings/json-inspector/internal/domain'
import { t as tr } from '../i18n'
import type { EnvScope, EnvState, Environment, Variable } from '../../bindings/json-inspector/internal/domain'
import type { ImportReport } from '../../bindings/json-inspector/internal/usecase/environment'

// What the environments lived in before they moved into the database.
const LEGACY_KEY = 'ji-env-v1'

/** One row of the `.env` import dialog: what was found, and what to do with it. */
export interface ImportChoice {
  name: string
  value: string
  secret: boolean
  mode: 'replace' | 'skip'
}

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
    // Session-only: an unlock must not outlive the sheet that made it.
    unlockedEnvIds: [] as string[],
    sheetOpen: false,
    sheetFocus: null as { envId: string | null; varName: string } | null,
    editedEnvId: null as string | null,
    // Values the user asked to see, by variable id. Filled by reveal() alone, dropped when the
    // sheet closes: a secret is not carried around just because it exists.
    revealed: {} as Record<string, string>,
    importReport: null as ImportReport | null,
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
    // left, and so does the unlock the user gave one of them for this session. A revealed secret is
    // dropped for the same reason — it is a value of a variable that is no longer there.
    forget() {
      this.envState = null
      this.unlockedEnvIds = []
      this.revealed = {}
      this.editedEnvId = null
      this.sheetFocus = null
      this.importReport = null
    },

    // The old build kept environments in localStorage and their secrets in the OS keychain; both
    // move into the database on the first launch of this one. The raw string goes over as it is —
    // reading that shape is Go's job — and the key is dropped only once the import is through.
    async importLegacyOnce() {
      const raw = localStorage.getItem(LEGACY_KEY)
      if (raw === null) return
      try {
        this.importReport = await EnvironmentsService.ImportLegacy(raw)
        localStorage.removeItem(LEGACY_KEY)
        this.envState = await EnvironmentsService.Snapshot()
      } catch {
        // The key stays: the next launch tries again, which is how a database that was locked or
        // read-only recovers once the user fixes it.
      }
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

    async addEnv(name = tr('environments.new')): Promise<string> {
      this.envState = await EnvironmentsService.CreateEnvironment(name)
      return this.activeId ?? ''
    },

    async removeEnv(id: string) {
      this.envState = await EnvironmentsService.DeleteEnvironment(id)
      this.unlockedEnvIds = this.unlockedEnvIds.filter((x) => x !== id)
      // Values revealed for a variable that no longer exists go with it.
      const live = new Set(this.environments.flatMap((e) => e.vars.map((v) => v.id)))
      for (const key of Object.keys(this.revealed)) {
        if (!live.has(key)) delete this.revealed[key]
      }
    },

    async renameEnv(id: string, name: string) {
      this.envState = await EnvironmentsService.UpdateEnvironment(id, { name })
    },

    async setEnvReadonly(id: string, readonly: boolean) {
      this.envState = await EnvironmentsService.UpdateEnvironment(id, { readonly })
    },

    // ---- variables -------------------------------------------------------

    // The sheet's "add row" leaves this empty; the .env dialog fills it in. The new row lands last
    // in its scope, which is what addVar's caller gets back.
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

    async importDotenv(envId: string | null, entries: ImportChoice[]) {
      // The dialog decides what happens to a name that is already there; Go merges what it is given.
      const kept = entries
        .filter((e) => e.mode === 'replace' && e.name.trim() !== '')
        .map((e) => ({ name: e.name.trim(), value: e.value, secret: e.secret }))
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

    resolveVariable(name: string): VarResolution | null {
      const env = this.activeEnvironment
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
      this.unlockedEnvIds = []
      // Everything the user revealed was revealed for that visit.
      this.revealed = {}
    },

    isUnlocked(envId: string | null): boolean {
      return envId !== null && this.unlockedEnvIds.includes(envId)
    },

    unlock(envId: string) {
      if (!this.unlockedEnvIds.includes(envId)) this.unlockedEnvIds.push(envId)
    },
  },
})
