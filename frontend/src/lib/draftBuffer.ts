import { TextField, type TextResult } from '../../bindings/json-inspector/internal/usecase/draft'

// The two texts a request store is typing, and the revisions that say whose answer is whose.
//
// The command line and a collection card each hold one of these, and the rules are the same for both:
// what the window is typing is the window's until its own flush comes back, so an answer to a
// keystroke that has since been typed over is dropped rather than applied. Two copies of that rule
// would be two places for it to be wrong — and the revision counting is the part nobody gets right
// twice — so a store holds the fields and this file holds what may be done to them.
export interface DraftBuffer {
  urlText: string
  bodyText: string
  urlRev: number
  bodyRev: number
  bufferedUrl: boolean
  bufferedBody: boolean
  flushTimer: ReturnType<typeof setTimeout> | null
}

// How long the window holds a text it is typing before handing it over. The draft is Go's, so every
// keystroke would otherwise be a write on the other side of the boundary; anything that ends the
// moment — blur, Enter, sending — flushes at once, which is why this is a pause and not a schedule.
export const FLUSH_MS = 400

// One text the window is still holding. `rev` travels with it so the answer can be recognised as an
// answer to this keystroke and not to the one before it.
export interface Owed {
  field: TextField
  text: string
  rev: number
}

// A keystroke. The text is kept and the hand-over is put off; `flush` is the store's own, and is what
// the wait is for — this file does not know how a text is sent, only that one is owed.
export function typed(
  buffer: DraftBuffer,
  field: TextField,
  text: string,
  flush: () => void
): void {
  if (field === TextField.FieldURL) {
    buffer.urlText = text
    buffer.urlRev += 1
    buffer.bufferedUrl = true
  } else {
    buffer.bodyText = text
    buffer.bodyRev += 1
    buffer.bufferedBody = true
  }
  if (buffer.flushTimer) clearTimeout(buffer.flushTimer)
  buffer.flushTimer = setTimeout(flush, FLUSH_MS)
}

// What the window is still holding, and the end of the wait. A text stays owed until its own answer
// comes back — a flush that failed leaves it owed, so the next one sends it again rather than losing
// what was typed.
export function owed(buffer: DraftBuffer): Owed[] {
  if (buffer.flushTimer) {
    clearTimeout(buffer.flushTimer)
    buffer.flushTimer = null
  }
  const out: Owed[] = []
  if (buffer.bufferedUrl) {
    out.push({ field: TextField.FieldURL, text: buffer.urlText, rev: buffer.urlRev })
  }
  if (buffer.bufferedBody) {
    out.push({ field: TextField.FieldBody, text: buffer.bodyText, rev: buffer.bodyRev })
  }
  return out
}

// Whether an answer belongs to the text the window is holding now, and the release of it: a reply to
// a keystroke that has since been typed over describes a text nobody is looking at, and one that
// matches says the window and Go agree again.
export function settled(buffer: DraftBuffer, result: TextResult): boolean {
  if (result.field === TextField.FieldURL) {
    if (result.rev !== buffer.urlRev) return false
    buffer.bufferedUrl = false
    return true
  }
  if (result.rev !== buffer.bodyRev) return false
  buffer.bufferedBody = false
  return true
}

// Abandoning the wait without sending anything: what a caller does when the text is about to be
// replaced, so a flush of the text being replaced cannot land on the one that replaced it.
export function abandoned(buffer: DraftBuffer): void {
  if (buffer.flushTimer) clearTimeout(buffer.flushTimer)
  buffer.flushTimer = null
  buffer.bufferedUrl = false
  buffer.bufferedBody = false
}
