import type { Auth, BodyKind, CookieRow, FormRow, Row, RowKind, Scripts } from '../../bindings/json-inspector/internal/domain'
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
  // The format the body is composed in, and what it is composed of when it is not text. All three
  // travel together: the format decides which of them the request goes out as, and switching between
  // them must not lose the others.
  bodyKind: BodyKind
  form: FormRow[]
  bodyFile: string
  params: Row[]
  headers: Row[]
  cookies: CookieRow[]
  auth: Auth
  // Whether this request can take its authorization from the levels above it. A card inside a
  // collection can — the chip offers «Наследовать» — and the command line cannot, because nothing is
  // above it.
  canInherit: boolean
  // What those levels answered, for the chip to say what inheriting would mean here.
  inheritedAuth: Auth | null
  missingVars: string[]
  enabledParamsCount: number
  enabledHeadersCount: number
  // Whether the body chip has anything behind it. Both stores answer it the same way, so the dashed
  // chip cannot go solid in one place and not the other.
  hasBody: boolean
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
  setBodyKind(kind: BodyKind): Promise<void>
  setBodyFile(path: string): Promise<void>
  // Asks Go for a file and answers with its path, or with nothing when the dialog was closed. Go
  // reads the file at send time; the path is all the window ever holds.
  pickBodyFile(): Promise<string>
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
