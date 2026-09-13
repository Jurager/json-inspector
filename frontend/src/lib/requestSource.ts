import type { Auth, CookieRow, Row, RowKind, Scripts } from '../../bindings/json-inspector/internal/domain'
import type { RowPatch, Seed } from '../../bindings/json-inspector/internal/usecase/draft'

// What the request builder and the response viewer need from whichever store is showing them.
//
// The command line and a collection card compose the same thing — a method, an address, rows, a body
// — and the design asks for the same component in both places. This is the shape that makes them the
// same component: both stores answer it, so the builder never asks which one it is talking to.
export type ChipName = 'params' | 'headers' | 'auth' | 'body' | 'scripts'

export interface RequestSource {
  method: string
  url: string
  body: string
  params: Row[]
  headers: Row[]
  cookies: CookieRow[]
  auth: Auth
  missingVars: string[]
  enabledParamsCount: number
  enabledHeadersCount: number
  bodyDisabled: boolean
  loading: boolean
  openChip: ChipName | null
  // The code this request runs around itself: what it has of its own, and — through the chain — what
  // would run instead of it while that stays empty. Both come from Go; the editor owns the typing.
  scripts: Scripts | null
  // The level whose code belongs in the editor — the node a card opened, or the command line's draft —
  // and the level the answer in `scripts` is about. The two are equal once the answer has arrived.
  scriptsLevel: string | null
  scriptsFor: string | null
  inheritedScript(scope: 'pre' | 'post'): { text: string; name: string } | null
  loadScripts(): Promise<void>
  saveScripts(pre: string, post: string): Promise<void>

  setUrl(text: string): void
  setBody(text: string): void
  flush(): Promise<void>
  setMethod(method: string): Promise<void>
  setAuth(auth: Auth): Promise<void>
  addRow(kind: RowKind): Promise<string>
  removeRow(kind: RowKind, id: string): Promise<void>
  patchRow(kind: RowKind, id: string, patch: RowPatch): Promise<void>
  toggleRow(kind: RowKind, id: string, enabled: boolean): Promise<void>
  setOpenChip(chip: ChipName | null): void
  // A whole request handed over — a pasted command — replacing what is being composed.
  replace(seed: Seed): Promise<void>
  send(): Promise<void>
  cancel(): Promise<void>
  // A send that failed on this side has nothing to wait for any more.
  failSend(): void
}

// The parts of the response pane that belong to the window rather than to the request: which node of
// the document is open in the inspector, and how wide it is.
export interface InspectorHost {
  inspector: { open: boolean; path: string | null; width: number }
  setInspector(partial: Partial<{ open: boolean; path: string | null; width: number }>): void
}
