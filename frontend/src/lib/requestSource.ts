import { AuthType } from '../../bindings/json-inspector/internal/domain'
import type { Auth, AuthToken, BodyKind, CookieRow, FormRow, ProjectedRow, Row, RowKind, Scripts } from '../../bindings/json-inspector/internal/domain'
import type { CommandResult, RowPatch, Seed } from '../../bindings/json-inspector/internal/usecase/draft'

// A request nobody has authorized yet, and what every reader falls back on. «Нет» is an answer and
// not the absence of one, which is why it is written down rather than left undefined.
export const NO_AUTH: Auth = { type: AuthType.AuthNone, fields: {} }

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
  // The environment this one request resolves its `{{tokens}}` in, apart from the window's own. Empty
  // is "not pinned" and means it follows whatever the window is on — the state every request is in
  // until somebody picks one in the bar.
  environmentId: string
  setEnvironmentOverride(id: string): Promise<void>
  // The rows the authorization puts in those lists — a Bearer token is an Authorization header, an
  // API key is a header or a query parameter. They are worked out by Go on every answer and are not
  // stored anywhere: the scheme's fields are what is written down, and these are what they come to.
  projected: ProjectedRow[]
  // Which of them, by the list they are in and the name they carry. A row has no id to address: it
  // is not a row, it is what a field comes to, and the edit goes to the field behind it.
  patchDerived(target: RowKind, name: string, value: string): Promise<void>
  // Deleting one deletes what put it there: the request stops authorizing itself.
  removeDerived(): Promise<void>
  // The state of a credential somebody else issues, for the schemes that have one, and the two things
  // the design offers for them. The token itself never comes back: the window draws «нет токена» or
  // that there is one, and has no use for the value.
  token: AuthToken | null
  obtainAuth(): Promise<void>
  forgetAuth(): Promise<void>
  // Whether this request can take its authorization from the levels above it. A card inside a
  // collection can — the chip offers «Наследовать» — and the command line cannot, because nothing is
  // above it.
  canInherit: boolean
  // What those levels answered, for the chip to say what inheriting would mean here.
  inheritedAuth: Auth | null
  missingVars: string[]
  // Asking the draft side about them again. A variable added in the environments window is not an
  // edit of this request, so nothing here would otherwise hear that a name has stopped missing — and
  // the block that holds the send would sit over the button after the variable it names exists.
  refreshPreview(): Promise<void>
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
  // The two halves of code, and whether each is switched off. Code that is off still travels: it is
  // the level's own answer about itself, and throwing it away would be deleting it.
  saveScripts(pre: string, post: string, off: { pre: boolean; post: boolean }): Promise<void>

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
  // A whole request handed over — a record opened in the line — replacing what is being composed.
  replace(seed: Seed): Promise<void>
  // A command pasted into the line: read on the other side, and the draft replaced with what it
  // came to. The reading is returned so the window can word the toast; when it is not a command at
  // all, nothing was replaced and the caller puts the text in the field itself.
  pasteCommand(text: string): Promise<CommandResult>
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
