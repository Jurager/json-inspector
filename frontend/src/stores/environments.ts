import { defineStore } from 'pinia'
import {
  missingTokens,
  substituteTokens,
  substituteTokensMasked,
  type VarKind,
  type VarResolution,
} from '../lib/vars'
import type { DotenvEntry } from '../lib/dotenv'
import { App as Backend } from '../../bindings/json-inspector'

export interface Variable {
  id: string
  name: string
  value: string
  kind: VarKind
  enabled: boolean
}

export interface InheritedVariable extends Variable {
  overridden: boolean
}

export interface Environment {
  id: string
  name: string
  color?: 'green' | 'orange' | 'red' | 'purple'
  readonly: boolean
  vars: Variable[]
}

interface Persisted {
  environments: Environment[]
  globals: Variable[]
  activeId: string | null
}

const ENVIRONMENTS_STORAGE_KEY = 'ji-env-v1'

let seq = 0
function nextId(prefix: string): string {
  seq += 1
  return `${prefix}-${Date.now().toString(36)}-${seq}`
}

function newVar(name = '', value = '', kind: VarKind = 'text'): Variable {
  return { id: nextId('var'), name, value, kind, enabled: true }
}

function defaultState(): Persisted {
  const local: Environment = {
    id: nextId('env'),
    name: 'Local · dev',
    readonly: false,
    vars: [newVar('baseUrl', 'http://localhost:8000')],
  }
  return { environments: [local], globals: [], activeId: local.id }
}

function loadState(): Persisted {
  try {
    const raw = localStorage.getItem(ENVIRONMENTS_STORAGE_KEY)
    if (!raw) return defaultState()
    const parsed = JSON.parse(raw)
    if (!parsed || !Array.isArray(parsed.environments) || !Array.isArray(parsed.globals)) {
      return defaultState()
    }
    return {
      environments: parsed.environments,
      globals: parsed.globals,
      activeId: typeof parsed.activeId === 'string' ? parsed.activeId : null,
    }
  } catch {
    // Corrupt or unavailable storage shouldn't cost the user the feature.
    return defaultState()
  }
}

export interface ImportChoice extends DotenvEntry {
  mode: 'replace' | 'skip'
}

export const useEnvironmentsStore = defineStore('environments', {
  state: () => ({
    ...loadState(),
    // Session-only: an unlock must not outlive the sheet that made it.
    unlockedEnvIds: [] as string[],
    sheetOpen: false,
    sheetFocus: null as { envId: string | null; varName: string } | null,
    editedEnvId: null as string | null,
    secretValues: {} as Record<string, string>,
    isKeychainAvailable: true,
  }),

  getters: {
    activeEnvironment(state): Environment | null {
      return state.environments.find((e) => e.id === state.activeId) ?? null
    },
  },

  actions: {
    // Shared by the substitution actions and the token tooltips: environment first, then
    // globals, and a disabled variable never participates.
    resolveVariable(name: string): VarResolution | null {
      const env = this.activeEnvironment
      if (env) {
        const v = env.vars.find((x) => x.name === name && x.enabled)
        if (v) return { value: this.effectiveValue(env.id, v), source: 'env', kind: v.kind }
      }
      const g = this.globals.find((x) => x.name === name && x.enabled)
      if (g) return { value: this.effectiveValue(null, g), source: 'global', kind: g.kind }
      return null
    },

    // A secret's value lives in the keychain-backed map, not in the model.
    effectiveValue(envId: string | null, v: Variable): string {
      return v.kind === 'secret' ? (this.secretValues[secretKey(envId, v.name)] ?? '') : v.value
    },

    substitute(text: string): string {
      return substituteTokens(text, this.resolveVariable)
    },

    maskSecrets(text: string): string {
      return substituteTokensMasked(text, this.resolveVariable)
    },

    missingVarNames(text: string): string[] {
      return missingTokens(text, this.resolveVariable)
    },

    rowsFor(envId: string | null): { own: Variable[]; inherited: InheritedVariable[] } {
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

    persist() {
      try {
        const data: Persisted = {
          environments: this.environments,
          globals: this.globals,
          activeId: this.activeId,
        }
        localStorage.setItem(ENVIRONMENTS_STORAGE_KEY, JSON.stringify(data))
      } catch {
        // ignore quota/availability errors — losing persistence beats losing the app
      }
    },

    setActive(id: string | null) {
      this.activeId = id
      this.persist()
    },

    addEnv(name = 'Новое окружение'): string {
      const env: Environment = { id: nextId('env'), name, readonly: false, vars: [] }
      this.environments.push(env)
      this.persist()
      return env.id
    },

    removeEnv(id: string) {
      const doomed = this.environments.find((e) => e.id === id)
      this.environments = this.environments.filter((e) => e.id !== id)
      // Falls back to "Без окружения" rather than promoting a neighbour the user didn't choose.
      if (this.activeId === id) this.activeId = null
      this.unlockedEnvIds = this.unlockedEnvIds.filter((x) => x !== id)
      for (const k of Object.keys(this.secretValues)) {
        if (k.startsWith(id + ':')) delete this.secretValues[k]
      }
      // A deleted environment must not leave credentials behind in the keychain.
      for (const v of doomed?.vars ?? []) {
        if (v.kind === 'secret') this.deleteSecretFromKeychain(id, v.name)
      }
      this.persist()
    },

    renameEnv(id: string, name: string) {
      const env = this.environments.find((e) => e.id === id)
      if (!env) return
      env.name = name
      this.persist()
    },

    setEnvReadonly(id: string, readonly: boolean) {
      const env = this.environments.find((e) => e.id === id)
      if (!env) return
      env.readonly = readonly
      this.persist()
    },

    addVar(envId: string | null, init: Partial<Variable> = {}): string {
      const v: Variable = {
        id: nextId('var'),
        name: init.name ?? '',
        value: init.value ?? '',
        kind: init.kind ?? 'text',
        enabled: init.enabled ?? true,
      }
      this.varsOf(envId).push(v)
      this.persist()
      return v.id
    },

    updateVar(envId: string | null, varId: string, patch: Partial<Variable>) {
      const v = this.varsOf(envId).find((x) => x.id === varId)
      if (!v) return
      const wasKind = v.kind
      const wasName = v.name
      const nextName = patch.name ?? v.name
      const nextKind = patch.kind ?? v.kind
      const key = secretKey(envId, wasName)
      const nextKey = secretKey(envId, nextName)

      const renamingSecret = wasKind === 'secret' && nextKind === 'secret' && nextName !== wasName
      let carried: string | undefined

      if (renamingSecret) {
        // Carried along, or the vault entry is orphaned and the variable reads as empty.
        carried = this.secretValues[key] ?? ''
        this.secretValues[nextKey] = carried
        delete this.secretValues[key]
      } else if (wasKind !== 'secret' && nextKind === 'secret') {
        // Promotion: the plaintext leaves the model entirely.
        this.secretValues[nextKey] = v.value
        carried = v.value
      } else if (wasKind === 'secret' && nextKind !== 'secret') {
        // Demotion: hand the value back to the model so the field isn't blank.
        if (patch.value === undefined) patch = { ...patch, value: this.secretValues[key] ?? '' }
        delete this.secretValues[key]
      }

      const clean = { ...patch }
      let written = carried
      if (nextKind === 'secret') {
        if (clean.value !== undefined) {
          this.secretValues[nextKey] = clean.value
          written = clean.value
        }
        clean.value = ''
      }
      Object.assign(v, clean)

      // The keychain is the only home for a secret's value: a rename must rewrite it (entries
      // are keyed by name) and a demotion must clear it, or the credential stays alive.
      if (nextKind === 'secret' && written !== undefined) {
        this.writeSecretToKeychain(envId, nextName, written)
      }
      if (wasKind === 'secret' && (nextKind !== 'secret' || renamingSecret)) {
        this.deleteSecretFromKeychain(envId, wasName)
      }
      this.persist()
    },

    removeVar(envId: string | null, varId: string) {
      const list = this.varsOf(envId)
      const v = list.find((x) => x.id === varId)
      if (!v) return
      if (v.kind === 'secret') {
        delete this.secretValues[secretKey(envId, v.name)]
        this.deleteSecretFromKeychain(envId, v.name)
      }
      const at = list.indexOf(v)
      if (at !== -1) list.splice(at, 1)
      this.persist()
    },

    setSecret(envId: string | null, name: string, value: string) {
      this.secretValues[secretKey(envId, name)] = value
      this.writeSecretToKeychain(envId, name, value)
    },

    writeSecretToKeychain(envId: string | null, name: string, value: string) {
      try {
        Backend.SecretSet(envId ?? 'globals', name, value).catch(() => {
          this.isKeychainAvailable = false
        })
      } catch {
        this.isKeychainAvailable = false
      }
    },

    deleteSecretFromKeychain(envId: string | null, name: string) {
      try {
        Backend.SecretDelete(envId ?? 'globals', name).catch(() => {
          this.isKeychainAvailable = false
        })
      } catch {
        this.isKeychainAvailable = false
      }
    },

    async hydrateSecrets() {
      const targets: { envId: string | null; name: string }[] = []
      for (const e of this.environments) {
        for (const v of e.vars) if (v.kind === 'secret') targets.push({ envId: e.id, name: v.name })
      }
      for (const v of this.globals) if (v.kind === 'secret') targets.push({ envId: null, name: v.name })

      for (const t of targets) {
        try {
          const value = await Backend.SecretGet(t.envId ?? 'globals', t.name)
          if (value) this.secretValues[secretKey(t.envId, t.name)] = value
        } catch {
          // One failure is enough to know this machine can't store secrets.
          this.isKeychainAvailable = false
          return
        }
      }
    },

    editEnv(id: string | null) {
      this.editedEnvId = id
    },

    importDotenv(envId: string | null, entries: ImportChoice[]) {
      for (const e of entries) {
        const existing = this.varsOf(envId).find((v) => v.name === e.name)
        if (existing) {
          if (e.mode === 'skip') continue
          this.updateVar(envId, existing.id, {
            value: e.value,
            kind: e.secret ? 'secret' : existing.kind,
          })
          continue
        }
        const id = this.addVar(envId, { name: e.name, kind: e.secret ? 'secret' : 'text' })
        this.updateVar(envId, id, { value: e.value })
      }
      this.persist()
    },

    openSheet(focus: { envId: string | null; varName: string } | null = null) {
      this.sheetFocus = focus
      // Opening on a variable implies its environment; otherwise it starts on the active one.
      this.editedEnvId = focus ? focus.envId : this.activeId
      this.sheetOpen = true
    },

    closeSheet() {
      this.sheetOpen = false
      this.sheetFocus = null
      this.unlockedEnvIds = []
    },

    /** Read-only environments are edited only after an explicit unlock. */
    isUnlocked(envId: string | null): boolean {
      return envId !== null && this.unlockedEnvIds.includes(envId)
    },

    unlock(envId: string) {
      if (!this.unlockedEnvIds.includes(envId)) this.unlockedEnvIds.push(envId)
    },
  },
})

function secretKey(envId: string | null, name: string): string {
  return `${envId ?? 'globals'}:${name}`
}
