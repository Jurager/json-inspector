// Environment tokens: `{{name}}`, spaces allowed inside, `\{{` escapes the sequence. Only
// a valid variable name counts, so payloads like `{{"a": 1}}` pass through untouched.

export type VarKind = 'text' | 'secret'

// What a secret looks like anywhere it isn't deliberately revealed: tooltips, preview, exports.
export const SECRET_MASK = '••••'

// `source` and `kind` travel with the value: the tooltip names where it came from and whether it
// may be shown.
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

// Unknown tokens are left exactly as written — `missingTokens` reports them and blocks
// sending, so blanking them here would hide the mistake instead of surfacing it.
export function substituteTokens(text: string, resolve: ResolveFn): string {
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

// Names that appear as tokens but resolve to nothing, deduplicated, in order of first appearance.
export function missingTokens(text: string, resolve: ResolveFn): string[] {
  const names = new Set<string>()
  for (const t of parseTokens(text)) {
    if (!resolve(t.name)) names.add(t.name)
  }
  return Array.from(names)
}

// Substitution for anything that outlives the moment of sending — the request preview, an
// export: a secret leaves as the mask, never as the value.
export function substituteTokensMasked(text: string, resolve: ResolveFn): string {
  const tokens = parseTokens(text)
  if (tokens.length === 0) return text
  let out = ''
  let last = 0
  for (const t of tokens) {
    const r = resolve(t.name)
    out += text.slice(last, t.start)
    out += r ? (r.kind === 'secret' ? SECRET_MASK : r.value) : t.raw
    last = t.end
  }
  return out + text.slice(last)
}

export interface TokenSegment {
  text: string
  tokenName?: string
  // The highlight layer sits over a real input, so a token click must translate back into a caret
  // index.
  start: number
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
