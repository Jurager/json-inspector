// Template tokens for environment variables: `{{name}}` (spaces inside are
// allowed). `\{{` escapes the sequence so it is never treated as a token.
//
// A token is only recognised when its contents are a valid variable name, so
// arbitrary payloads — a body containing `{{"a": 1}}`, a CSS-ish `{{nested}}`
// inside a string — pass through untouched instead of blowing up the parse.

export type VarKind = 'text' | 'secret'

// What a secret looks like anywhere it isn't being deliberately revealed:
// tooltips, the request preview, exports.
export const SECRET_MASK = '••••'

// What a resolver hands back for one name. `source` and `kind` travel with the
// value because the tooltip has to name where a value came from ("Local · dev →
// baseUrl") and has to know whether it may show it at all.
export interface VarResolution {
  value: string
  source: 'env' | 'global'
  kind: VarKind
}

export type ResolveFn = (name: string) => VarResolution | null

export interface Token {
  start: number
  end: number
  name: string
  raw: string
}

const NAME_RE = /^[A-Za-z_][A-Za-z0-9_]*$/

export function parseTokens(text: string): Token[] {
  const out: Token[] = []
  let i = 0
  while (i < text.length) {
    // Skip an escaped opener whole (backslash + both braces): consuming all
    // three is what keeps the braces of `\{{x}}` from opening a token.
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
      // Contents aren't a name — this is content that merely looks like a
      // token. Resume after the opener so a real token further along is still
      // found, rather than swallowing the rest of the string.
      i += 2
      continue
    }
    out.push({ start: i, end: close + 2, name, raw: text.slice(i, close + 2) })
    i = close + 2
  }
  return out
}

// Replaces known tokens with their values. Unknown ones are left exactly as
// written — an unresolved name is reported by `missing` and blocks sending,
// so silently blanking it here would hide the mistake instead of surfacing it.
export function substitute(text: string, resolve: ResolveFn): string {
  const tokens = parseTokens(text)
  if (tokens.length === 0) return text
  let out = ''
  let last = 0
  for (const t of tokens) {
    const r = resolve(t.name)
    out += text.slice(last, t.start)
    out += r ? r.value : t.raw
    last = t.end
  }
  return out + text.slice(last)
}

// Names that appear as tokens but resolve to nothing, deduplicated and in order
// of first appearance — the list the "переменная не найдена" row and the
// status bar both need.
export function missing(text: string, resolve: ResolveFn): string[] {
  const names = new Set<string>()
  for (const t of parseTokens(text)) {
    if (!resolve(t.name)) names.add(t.name)
  }
  return Array.from(names)
}

export interface Segment {
  text: string
  token?: string
  // Where the segment starts in the source text. The highlight layer sits over a
  // real input, so a token click has to translate back into a caret index.
  start: number
}

// Splits text into literal runs and tokens for the highlight layers. Concatenating
// the segments reproduces the input exactly — the display layers paint over a
// real input, so a character dropped here would show up as misaligned text.
export function segments(text: string): Segment[] {
  const out: Segment[] = []
  let last = 0
  for (const t of parseTokens(text)) {
    if (t.start > last) out.push({ text: text.slice(last, t.start), start: last })
    out.push({ text: t.raw, token: t.name, start: t.start })
    last = t.end
  }
  if (last < text.length) out.push({ text: text.slice(last), start: last })
  return out
}
