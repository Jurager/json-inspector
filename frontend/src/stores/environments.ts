import { defineStore } from 'pinia'
import {
  missing as missingTokens,
  parseTokens,
  substitute as substituteTokens,
  SECRET_MASK,
  type ResolveFn,
  type VarKind,
} from '../lib/vars'
import { SecretSet, SecretGet, SecretDelete } from '../../wailsjs/go/main/App'

export interface Variable {
  id: string
  name: string
  // Empty for secrets by construction: a secret's value lives in `secretValues`
  // (memory only), so it can never reach localStorage or an export by someone
  // forgetting to strip it.
  value: string
  kind: VarKind
  enabled: boolean
}

export interface Environment {
  id: string
  name: string
  color?: 'green' | 'orange' | 'red' | 'purple'
  // Prod-like environments are editable only after an explicit unlock.
  readonly: boolean
  vars: Variable[]
}

// What gets written to localStorage. Deliberately narrower than the state:
// session-only things (`unlocked`, the open sheet, the secret vault) are not
// part of it, so they can't be persisted by accident.
interface Persisted {
  environments: Environment[]
  globals: Variable[]
  activeId: string | null
}

const STORAGE_KEY = 'ji-env-v1'

let seq = 0
function nextId(prefix: string): string {
  seq += 1
  return `${prefix}-${Date.now().toString(36)}-${seq}`
}

function newVar(name = '', value = '', kind: VarKind = 'text'): Variable {
  return { id: nextId('var'), name, value, kind, enabled: true }
}

function defaultState(): Persisted {
  // The one environment a fresh install needs to make `{{baseUrl}}` mean
  // something. Prod-like environments are never created for the user — those
  // carry real credentials and must be an explicit decision.
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
    const raw = localStorage.getItem(STORAGE_KEY)
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

export const useEnvironmentsStore = defineStore('environments', {
  state: () => ({
    ...loadState(),
    // Session-only: an unlocked read-only environment locks again when the
    // sheet closes, so the unlock can't outlive the editing session.
    unlocked: [] as string[],
    sheetOpen: false,
    sheetFocus: null as { envId: string | null; varName: string } | null,
    // Which environment the sheet's table is showing. Separate from `activeId`
    // on purpose: editing an environment shouldn't silently change what the
    // whole window substitutes.
    sheetEnvId: null as string | null,
    // Secret values, keyed `<envId|globals>:<name>`. Never serialised; the
    // keychain refills it on startup.
    secretValues: {} as Record<string, string>,
    // False once a keychain call has failed, which is the honest answer on a
    // platform without one (or when access was denied). The app keeps working
    // with in-memory secrets and the editor says they won't survive a restart.
    keychainAvailable: true,
  }),

  getters: {
    active(state): Environment | null {
      return state.environments.find((e) => e.id === state.activeId) ?? null
    },

    // The resolver the substitution core and the token tooltips share. Precedence
    // is "request → environment → globals"; the request tier has no store of its
    // own (a value typed straight into the URL simply isn't a token), so what's
    // implemented here is environment-then-globals — first match wins, and a
    // disabled variable never participates.
    resolve(): ResolveFn {
      // A secret keeps its value beside the model, never inside it — so reading
      // one goes through the vault rather than the variable.
      const valueOf = (envId: string | null, v: Variable): string =>
        v.kind === 'secret' ? (this.secretValues[secretKey(envId, v.name)] ?? '') : v.value

      return (name: string) => {
        const env = this.active
        if (env) {
          const v = env.vars.find((x) => x.name === name && x.enabled)
          if (v) return { value: valueOf(env.id, v), source: 'env' as const, kind: v.kind }
        }
        const g = this.globals.find((x) => x.name === name && x.enabled)
        if (g) return { value: valueOf(null, g), source: 'global' as const, kind: g.kind }
        return null
      }
    },

    missingIn(): (text: string) => string[] {
      return (text: string) => missingTokens(text, this.resolve)
    },

    substitute(): (text: string) => string {
      return (text: string) => substituteTokens(text, this.resolve)
    },

    // Same substitution, but a secret comes out as dots. Used for anything that
    // outlives the moment of sending — the request preview and the exports —
    // so a credential can't ride along in a screenshot or a copied snippet.
    masked(): (text: string) => string {
      return (text: string) => {
        const tokens = parseTokens(text)
        if (tokens.length === 0) return text
        let out = ''
        let last = 0
        for (const t of tokens) {
          const r = this.resolve(t.name)
          out += text.slice(last, t.start)
          out += r ? (r.kind === 'secret' ? SECRET_MASK : r.value) : t.raw
          last = t.end
        }
        return out + text.slice(last)
      }
    },
  },

  actions: {
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
        localStorage.setItem(STORAGE_KEY, JSON.stringify(data))
      } catch {
        // ignore quota/availability errors — losing persistence is better than
        // losing the app
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
      // Dropping the active environment falls back to "Без окружения" rather
      // than silently promoting a neighbour the user didn't choose.
      if (this.activeId === id) this.activeId = null
      this.unlocked = this.unlocked.filter((x) => x !== id)
      for (const k of Object.keys(this.secretValues)) {
        if (k.startsWith(id + ':')) delete this.secretValues[k]
      }
      // An environment's secrets go with it — a deleted environment must not
      // leave credentials behind in the keychain.
      for (const v of doomed?.vars ?? []) {
        if (v.kind === 'secret') this.dropSecret(id, v.name)
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
        // A renamed secret takes its value along, or the vault entry is orphaned
        // and the variable silently reads as empty.
        carried = this.secretValues[key] ?? ''
        this.secretValues[nextKey] = carried
        delete this.secretValues[key]
      } else if (wasKind !== 'secret' && nextKind === 'secret') {
        // Promotion: the plaintext leaves the model entirely.
        this.secretValues[nextKey] = v.value
        carried = v.value
      } else if (wasKind === 'secret' && nextKind !== 'secret') {
        // Demotion: hand the value back to the model so the field isn't
        // mysteriously blank, and drop the vault entry.
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

      // Keep the keychain in step with the model: a secret's value belongs
      // there and nowhere else, and leaving it behind after a demotion would
      // keep a credential alive the user just turned into a plain field. A
      // rename has to rewrite it too — the keychain addresses items by name, so
      // moving the vault entry alone would strand the stored copy under the old
      // name and lose the value on the next start.
      if (nextKind === 'secret' && written !== undefined) {
        this.storeSecret(envId, nextName, written)
      }
      if (wasKind === 'secret' && (nextKind !== 'secret' || renamingSecret)) {
        this.dropSecret(envId, wasName)
      }
      this.persist()
    },

    removeVar(envId: string | null, varId: string) {
      const list = this.varsOf(envId)
      const v = list.find((x) => x.id === varId)
      if (!v) return
      if (v.kind === 'secret') {
        delete this.secretValues[secretKey(envId, v.name)]
        this.dropSecret(envId, v.name)
      }
      const at = list.indexOf(v)
      if (at !== -1) list.splice(at, 1)
      this.persist()
    },

    setSecret(envId: string | null, name: string, value: string) {
      this.secretValues[secretKey(envId, name)] = value
      this.storeSecret(envId, name, value)
    },

    // The keychain lives on the Go side. Writes are fire-and-forget so the
    // editor never blocks on a system dialog; a failure flips `keychainAvailable`
    // and the footer stops promising persistence.
    storeSecret(envId: string | null, name: string, value: string) {
      try {
        SecretSet(envId ?? 'globals', name, value).catch(() => {
          this.keychainAvailable = false
        })
      } catch {
        this.keychainAvailable = false
      }
    },

    dropSecret(envId: string | null, name: string) {
      try {
        SecretDelete(envId ?? 'globals', name).catch(() => {
          this.keychainAvailable = false
        })
      } catch {
        this.keychainAvailable = false
      }
    },

    // Pulls stored secrets back into the session. Called once at startup,
    // before the first request can need one.
    async hydrateSecrets() {
      const targets: { envId: string | null; name: string }[] = []
      for (const e of this.environments) {
        for (const v of e.vars) if (v.kind === 'secret') targets.push({ envId: e.id, name: v.name })
      }
      for (const v of this.globals) if (v.kind === 'secret') targets.push({ envId: null, name: v.name })

      for (const t of targets) {
        try {
          const value = await SecretGet(t.envId ?? 'globals', t.name)
          if (value) this.secretValues[secretKey(t.envId, t.name)] = value
        } catch {
          // One failure is enough to know this machine can't store secrets.
          this.keychainAvailable = false
          return
        }
      }
    },

    // The value a cell should edit — including the vault, which the model
    // deliberately doesn't hold.
    varValue(envId: string | null, v: Variable): string {
      return v.kind === 'secret' ? (this.secretValues[secretKey(envId, v.name)] ?? '') : v.value
    },

    selectSheetEnv(id: string | null) {
      this.sheetEnvId = id
    },

    openSheet(focus: { envId: string | null; varName: string } | null = null) {
      this.sheetFocus = focus
      // Opening on a specific variable implies the environment it lives in;
      // otherwise the sheet starts on whatever the window is using.
      this.sheetEnvId = focus ? focus.envId : this.activeId
      this.sheetOpen = true
    },

    closeSheet() {
      this.sheetOpen = false
      this.sheetFocus = null
      // Read-only environments lock again when the sheet closes — the unlock is
      // scoped to one editing session, never to the app's lifetime.
      this.unlocked = []
    },

    unlock(envId: string) {
      if (!this.unlocked.includes(envId)) this.unlocked.push(envId)
    },
  },
})

// One key space for both scopes: globals have no environment id, so they get a
// literal one instead of a second map.
function secretKey(envId: string | null, name: string): string {
  return `${envId ?? 'globals'}:${name}`
}
