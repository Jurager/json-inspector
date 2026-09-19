// Environment tokens: `{{name}}`, spaces allowed inside, `\{{` escapes the sequence. Only
// a valid variable name counts, so payloads like `{{"a": 1}}` pass through untouched.
//
// What a token *is* is the window's business only as far as drawing it: what it comes to is resolved
// in Go, and what the window does with the answer is paint a pill over the characters. So this file
// stops at the token's shape — see internal/vars for the substitution itself.
import type { VariableKind } from '../../bindings/json-inspector/internal/domain'

// The kind of a variable is declared once, in Go, and reaches the window as a generated enum: a
// second spelling here is the duplicate this migration exists to remove.
export type VarKind = VariableKind

// `source` and `kind` travel with the value: the tooltip names where it came from and whether it
// may be shown.
export interface VarResolution {
  value: string
  source: 'env' | 'global'
  kind: VarKind
}

export interface Token {
  start: number
  end: number
  name: string
  raw: string
}

const NAME_RE = /^[A-Za-z_][A-Za-z0-9_]*$/

// The same rule Go's `validName` applies to a variable's name, and the same one a token has to
// satisfy to be found at all: a name outside it is one no `{{token}}` can carry. The window says so
// before the write rather than leaving the user with a row that quietly kept its old name.
export function isVariableName(name: string): boolean {
  return NAME_RE.test(name)
}

export function parseTokens(text: string): Token[] {
  const out: Token[] = []
  let i = 0
  while (i < text.length) {
    // Consume all three, or the braces of `\{{x}}` would open a token.
    if (text[i] === '\\' && text.startsWith('\\{{', i)) {
      i += 3
      continue
    }
    if (!text.startsWith('{{', i)) {
      i += 1
      continue
    }
    const close = text.indexOf('}}', i + 2)
    if (close === -1) {
      // No closer anywhere ahead, so no later token could close either.
      break
    }
    const name = text.slice(i + 2, close).trim()
    if (!NAME_RE.test(name)) {
      // Not a name — resume after the opener so a real token further along is still found.
      i += 2
      continue
    }
    out.push({ start: i, end: close + 2, name, raw: text.slice(i, close + 2) })
    i = close + 2
  }
  return out
}

export interface TokenSegment {
  text: string
  tokenName?: string
  // The highlight layer sits over a real input, so a token click must translate back into a caret
  // index.
  start: number
}

export interface UrlPiece {
  text: string
  /** Set when the piece is a `{{token}}` — that is what the pill is drawn for. */
  tokenName?: string
  /** How the piece is painted: a query is read in two inks, its names apart from its values. */
  kind?: 'query-key' | 'query-value'
  start: number
}

// Which ink each character of an address is written in. Everything before the `?` is the address
// itself; after it, a name runs up to its `=` and a value up to the next `&` — the grammar a query
// string is read by, and the one the design draws: `?include=` quiet, `author,comments` accented.
function charKinds(text: string): (UrlPiece['kind'])[] {
  const kinds: (UrlPiece['kind'])[] = new Array(text.length).fill(undefined)
  const query = text.indexOf('?')
  if (query < 0) return kinds

  let inValue = false
  for (let i = query; i < text.length; i++) {
    const char = text[i]
    if (char === '&') inValue = false
    kinds[i] = inValue ? 'query-value' : 'query-key'
    if (char === '=' && !inValue) inValue = true
  }
  return kinds
}

// The address as it is painted: its `{{tokens}}` and the two inks of its query, in the order they
// stand in the text. Concatenating the pieces reproduces the text exactly — the layer paints over a
// real input, so a dropped character would show up as the caret standing in the wrong place.
export function urlPieces(text: string): UrlPiece[] {
  const kinds = charKinds(text)
  const out: UrlPiece[] = []

  // A run of one kind, so that a long address is a handful of spans rather than one per character.
  function addPlain(from: number, to: number) {
    let start = from
    for (let i = from; i <= to; i++) {
      if (i === to || kinds[i] !== kinds[start]) {
        if (i > start) out.push({ text: text.slice(start, i), kind: kinds[start], start })
        start = i
      }
    }
  }

  let last = 0
  for (const token of parseTokens(text)) {
    addPlain(last, token.start)
    out.push({ text: token.raw, tokenName: token.name, start: token.start })
    last = token.end
  }
  addPlain(last, text.length)
  return out
}

// Concatenating the segments reproduces the input exactly: the layers paint over a real input,
// so a dropped character would show up as misaligned text.
export function tokenSegments(text: string): TokenSegment[] {
  const out: TokenSegment[] = []
  let last = 0
  for (const t of parseTokens(text)) {
    if (t.start > last) out.push({ text: text.slice(last, t.start), start: last })
    out.push({ text: t.raw, tokenName: t.name, start: t.start })
    last = t.end
  }
  if (last < text.length) out.push({ text: text.slice(last), start: last })
  return out
}
