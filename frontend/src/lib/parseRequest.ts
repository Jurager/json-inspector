import type { ExportFormat } from './export'

export type ParseErrorReason =
  | 'no-url'
  | 'bad-quotes'
  | 'leftover'
  | 'bad-fetch-init'
  | 'unsupported-variable'
  | 'unsupported-field'
  | 'unsupported-multipart'

export interface ParsedRequest {
  method: string
  url: string
  requestHeaders: Record<string, string>
  requestBody: string
}

export type ParseResult =
  | { kind: 'none' }
  | { kind: 'ok'; format: ExportFormat; request: ParsedRequest }
  | { kind: 'error'; reason: ParseErrorReason }

const METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']

interface Dialect {
  ps: boolean
  caret: boolean
}

const POSIX: Dialect = { ps: false, caret: false }
const SHELL: Dialect = { ps: false, caret: true }
const POWERSHELL: Dialect = { ps: true, caret: false }

function lineJoin(input: string, at: number, d: Dialect): number {
  const ch = input[at]
  const joins = d.ps ? ch === '`' : d.caret ? ch === '\\' || ch === '^' : ch === '\\'
  if (!joins) return -1
  let j = at + 1
  while (j < input.length && (input[j] === ' ' || input[j] === '\t')) j++
  return j < input.length && input[j] === '\n' ? j + 1 : -1
}

function readSingle(input: string, at: number, d: Dialect): { text: string; next: number } | null {
  let i = at + 1
  let text = ''
  while (i < input.length) {
    if (input[i] === "'") {
      if (d.ps && input[i + 1] === "'") {
        text += "'"
        i += 2
        continue
      }
      if (input[i + 1] === '\\' && input[i + 2] === "'" && input[i + 3] === "'") {
        text += "'"
        i += 4
        continue
      }
      return { text, next: i + 1 }
    }
    text += input[i]
    i++
  }
  return null
}

function readDouble(input: string, at: number, d: Dialect): { text: string; next: number } | null {
  const esc = d.ps ? '`' : '\\'
  let i = at + 1
  let text = ''
  while (i < input.length) {
    const c = input[i]
    if (c === esc) {
      const nx = input[i + 1]
      if (nx === undefined) return null
      if (nx === '\n') {
        i += 2
        continue
      }
      if (nx === '"' || nx === '\\' || nx === '$' || nx === '`') {
        text += nx
        i += 2
        continue
      }
      text += c
      i++
      continue
    }
    if (c === '"') return { text, next: i + 1 }
    text += c
    i++
  }
  return null
}

function readAnsiC(input: string, at: number): { text: string; next: number } | null {
  let i = at + 2
  let text = ''
  while (i < input.length) {
    const c = input[i]
    if (c === "'") return { text, next: i + 1 }
    if (c !== '\\') {
      text += c
      i++
      continue
    }
    const nx = input[i + 1]
    if (nx === undefined) return null
    if (nx === 'n') text += '\n'
    else if (nx === 't') text += '\t'
    else if (nx === 'r') text += '\r'
    else if (nx === 'u') text += String.fromCharCode(parseInt(input.slice(i + 2, i + 6), 16) || 0)
    else text += nx
    i += nx === 'u' ? 6 : 2
  }
  return null
}

function readHashtable(input: string, at: number): { text: string; next: number } | null {
  let i = at + 2
  let depth = 1
  while (i < input.length) {
    const c = input[i]
    if (c === "'" || c === '"') {
      const r = c === "'" ? readSingle(input, i, POWERSHELL) : readDouble(input, i, POWERSHELL)
      if (!r) return null
      i = r.next
      continue
    }
    if (c === '@' && input[i + 1] === '{') {
      depth++
      i += 2
      continue
    }
    if (c === '}') {
      depth--
      i++
      if (depth === 0) return { text: input.slice(at, i), next: i }
      continue
    }
    i++
  }
  return null
}

function tokenize(input: string, d: Dialect): string[] | null {
  const out: string[] = []
  let i = 0

  while (i < input.length) {
    const join = lineJoin(input, i, d)
    if (join !== -1) {
      i = join
      continue
    }
    if (/\s/.test(input[i]) || (!d.ps && input[i] === ';')) {
      i++
      continue
    }

    let token = ''
    while (i < input.length) {
      const inner = lineJoin(input, i, d)
      if (inner !== -1) {
        i = inner
        continue
      }
      const c = input[i]
      if (/\s/.test(c) || (!d.ps && c === ';')) break

      if (c === "'" || c === '"') {
        const r = c === "'" ? readSingle(input, i, d) : readDouble(input, i, d)
        if (!r) return null
        token += r.text
        i = r.next
        continue
      }
      if (c === '$' && input[i + 1] === "'") {
        const r = readAnsiC(input, i)
        if (!r) return null
        token += r.text
        i = r.next
        continue
      }
      if (c === '@' && input[i + 1] === '{' && d.ps) {
        const r = readHashtable(input, i)
        if (!r) return null
        token += r.text
        i = r.next
        continue
      }
      const esc = d.ps ? '`' : '\\'
      if (c === esc) {
        if (i + 1 >= input.length) return null
        token += input[i + 1]
        i += 2
        continue
      }
      token += c
      i++
    }
    out.push(token)
  }

  return out
}

function stripRedirection(text: string): string {
  let i = 0
  while (i < text.length) {
    const c = text[i]
    if (c === "'" || c === '"') {
      const r = c === "'" ? readSingle(text, i, POSIX) : readDouble(text, i, POSIX)
      i = r ? r.next : i + 1
      continue
    }
    if (c === '>' || c === '|') {
      // `2>&1` and `2> out` leave the file-descriptor number behind; it is not an argument.
      return text.slice(0, i).replace(/\s+\d+\s*$/, '')
    }
    i++
  }
  return text
}

/** Strip the things a copied command picks up that are not part of the command. */
function normalizeInput(text: string): string {
  return stripRedirection(
    text
      .replace(/^﻿/, '')
      .replace(/\r\n?/g, '\n')
      .replace(/^(?:\$\s+|PS\s[^>\n]*>\s*|>\s+|❯\s+)+/, '')
      .replace(/^sudo\s+/, '')
  )
}

function looksLikeUrl(s: string): boolean {
  if (/^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(s)) return true
  if (s.startsWith('/') || s.startsWith('{{')) return true
  if (/^localhost(:\d+)?(\/|$)/.test(s)) return true
  if (/^:\d+/.test(s)) return true
  return /^[^\s/:@]+\.[^\s/:@]/.test(s)
}

function isUrlOperand(s: string): boolean {
  if (/^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(s)) return true
  if (s.startsWith('/') || s.startsWith('{{')) return true
  return /^:\d+/.test(s) || /^localhost(:\d+)?(\/|$)/.test(s)
}

/** A value-taking flag placed *before* the URL would otherwise donate its value as the URL. */
const VALUE_FLAGS: Record<ExportFormat, Set<string>> = {
  curl: new Set([
    '-X', '--request', '-H', '--header', '-d', '--data', '--data-raw', '--data-binary',
    '--data-ascii', '--data-urlencode', '--url', '-u', '--user', '-A', '--user-agent',
    '-b', '--cookie', '-c', '--cookie-jar', '-e', '--referer', '-x', '--proxy', '-o',
    '--output', '-T', '--upload-file', '-F', '--form', '--form-string', '-m', '--max-time',
    '--connect-timeout', '--retry', '--resolve', '--cert', '--key', '--cacert', '--capath',
    '--interface', '--limit-rate', '-w', '--write-out', '-K', '--config', '--json',
    '--oauth2-bearer',
  ]),
  wget: new Set([
    '-O', '--output-document', '-o', '--output-file', '-P', '--directory-prefix', '--method',
    '--header', '--body-data', '--post-data', '--body-file', '--timeout', '-T', '--user',
    '--password', '--user-agent', '-U', '--referer', '--limit-rate',
  ]),
  httpie: new Set([
    '--auth', '-a', '--auth-type', '--bearer', '--timeout', '-o', '--output', '--session',
    '--pretty', '--style', '-p', '--print', '--verify', '--cert', '--cert-key', '--proxy',
    '--raw',
  ]),
  powershell: new Set([
    '-Method', '-Uri', '-Headers', '-Body', '-ContentType', '-TimeoutSec', '-Credential',
    '-UserAgent', '-Proxy', '-OutFile', '-MaximumRedirection', '-SessionVariable',
  ]),
  fetch: new Set(),
}

function splitFlag(token: string): [string, string | null] {
  if (!token.startsWith('-')) return [token, null]
  const eq = token.indexOf('=')
  return eq > 0 ? [token.slice(0, eq), token.slice(eq + 1)] : [token, null]
}

interface HeaderEntry {
  name: string
  value: string
}

function foldHeaders(entries: HeaderEntry[]): Record<string, string> {
  const map = new Map<string, HeaderEntry>()
  const cookies: string[] = []
  for (const e of entries) {
    if (e.name.toLowerCase() === 'cookie') {
      if (e.value) cookies.push(e.value)
      continue
    }
    map.set(e.name.toLowerCase(), e)
  }
  const out: Record<string, string> = {}
  for (const e of map.values()) out[e.name] = e.value
  if (cookies.length) out['Cookie'] = cookies.join('; ')
  return out
}

function parseHeaderArg(raw: string): HeaderEntry {
  const at = raw.search(/[:;]/)
  if (at === -1) return { name: raw.trim(), value: '' }
  return { name: raw.slice(0, at).trim(), value: raw.slice(at + 1).trim() }
}

function basicAuth(user: string, password: string): string {
  const bytes = new TextEncoder().encode(`${user}:${password}`)
  let binary = ''
  for (const b of bytes) binary += String.fromCharCode(b)
  return `Basic ${btoa(binary)}`
}

function pickUrl(positionals: string[]): string | null {
  return positionals.find(looksLikeUrl) ?? positionals[positionals.length - 1] ?? null
}

function hasLeftover(positionals: string[], url: string): boolean {
  const rest = [...positionals]
  const used = rest.indexOf(url)
  if (used !== -1) rest.splice(used, 1)
  return rest.length > 0
}

function appendQuery(url: string, query: string): string {
  if (!query) return url
  return url + (url.includes('?') ? '&' : '?') + query
}

function withFormType(entries: HeaderEntry[], hasBody: boolean): HeaderEntry[] {
  if (!hasBody) return entries
  if (entries.some((e) => e.name.toLowerCase() === 'content-type')) return entries
  return [...entries, { name: 'Content-Type', value: 'application/x-www-form-urlencoded' }]
}

function taker(tokens: string[], index: { i: number }, inline: string | null): string | null {
  if (inline !== null) return inline
  if (index.i + 1 < tokens.length) return tokens[++index.i]
  return null
}

function fromCurl(tokens: string[]): ParsedRequest | ParseErrorReason {
  let method: string | undefined
  let explicitUrl: string | undefined
  let user: string | undefined
  let nonBasicAuth = false
  let get = false
  const entries: HeaderEntry[] = []
  const data: string[] = []
  const forms: HeaderEntry[] = []
  const positionals: string[] = []

  const cursor = { i: 0 }
  for (cursor.i = 0; cursor.i < tokens.length; cursor.i++) {
    const raw = tokens[cursor.i]
    if (raw === '--') {
      positionals.push(...tokens.slice(cursor.i + 1))
      break
    }

    const [flag, inline] = splitFlag(raw)
    if (inline === null && !raw.startsWith('-')) {
      positionals.push(raw)
      continue
    }
    const take = () => taker(tokens, cursor, inline)

    switch (flag) {
      case '-X':
      case '--request':
        method = take()?.toUpperCase()
        break
      case '-H':
      case '--header': {
        const v = take()
        if (v) entries.push(parseHeaderArg(v))
        break
      }
      case '-d':
      case '--data':
      case '--data-raw':
      case '--data-binary':
      case '--data-ascii':
      case '--data-urlencode': {
        const v = take()
        if (v !== null) data.push(v)
        break
      }
      case '--json': {
        const v = take()
        if (v !== null) {
          data.push(v)
          entries.push({ name: 'Content-Type', value: 'application/json' })
        }
        break
      }
      case '--url':
        explicitUrl = take() ?? explicitUrl
        break
      case '-I':
      case '--head':
        method = 'HEAD'
        break
      case '-G':
      case '--get':
        get = true
        break
      case '-u':
      case '--user': {
        const v = take()
        if (v !== null) user = v
        break
      }
      case '-A':
      case '--user-agent': {
        const v = take()
        if (v !== null) entries.push({ name: 'User-Agent', value: v })
        break
      }
      case '-b':
      case '--cookie': {
        const v = take()
        // A value without `=` names a cookie *file*, not a cookie.
        if (v && v.includes('=')) entries.push({ name: 'Cookie', value: v })
        break
      }
      case '-F':
      case '--form':
      case '--form-string': {
        const v = take()
        if (v !== null) {
          const eq = v.indexOf('=')
          const name = eq === -1 ? v : v.slice(0, eq)
          const value = eq === -1 ? '' : v.slice(eq + 1)
          // `-F 'file=@path'` is an upload this app has no way to reproduce.
          if (value.startsWith('@') || value.startsWith('<')) return 'unsupported-multipart'
          forms.push({ name, value })
        }
        break
      }
      case '--digest':
      case '--ntlm':
      case '--negotiate':
      case '--anyauth':
      case '--proxy-anyauth':
        nonBasicAuth = true
        break
      default:
        if (VALUE_FLAGS.curl.has(flag)) take()
        break
    }
  }

  const url = explicitUrl ?? pickUrl(positionals)
  if (!url) return 'no-url'
  if (hasLeftover(positionals, url)) return 'leftover'

  if (user !== undefined && !nonBasicAuth) {
    const colon = user.indexOf(':')
    entries.push({
      name: 'Authorization',
      value: basicAuth(
        colon === -1 ? user : user.slice(0, colon),
        colon === -1 ? '' : user.slice(colon + 1)
      ),
    })
  }

  let body = data.join('&')
  if (!body && forms.length) body = forms.map((f) => `${f.name}=${f.value}`).join('&')

  return {
    method: method ?? (body && !get ? 'POST' : 'GET'),
    url: get && body ? appendQuery(url, body) : url,
    requestHeaders: foldHeaders(withFormType(entries, data.length > 0 || forms.length > 0)),
    requestBody: get ? '' : body,
  }
}

function fromWget(tokens: string[]): ParsedRequest | ParseErrorReason {
  let method: string | undefined
  let body = ''
  const entries: HeaderEntry[] = []
  const positionals: string[] = []

  const cursor = { i: 0 }
  for (cursor.i = 0; cursor.i < tokens.length; cursor.i++) {
    const raw = tokens[cursor.i]
    const [flag, inline] = splitFlag(raw)
    if (inline === null && !raw.startsWith('-')) {
      positionals.push(raw)
      continue
    }
    const take = () => taker(tokens, cursor, inline)

    switch (flag) {
      case '--method':
        method = take()?.toUpperCase()
        break
      case '--header': {
        const v = take()
        if (v) entries.push(parseHeaderArg(v))
        break
      }
      case '--body-data':
      case '--post-data':
        body = take() ?? ''
        break
      default:
        if (VALUE_FLAGS.wget.has(flag)) take()
        break
    }
  }

  const url = pickUrl(positionals)
  if (!url) return 'no-url'
  if (hasLeftover(positionals, url)) return 'leftover'

  return {
    method: method ?? (body ? 'POST' : 'GET'),
    url,
    requestHeaders: foldHeaders(withFormType(entries, body !== '')),
    requestBody: body,
  }
}

function httpieOperator(token: string): { op: string; at: number } | null {
  for (let i = 0; i < token.length; i++) {
    const two = token.slice(i, i + 2)
    if (two === ':=' || two === '==') return { op: two, at: i }
    if (token[i] === '=' || token[i] === ':' || token[i] === '@') return { op: token[i], at: i }
  }
  return null
}

function fromHttpie(tokens: string[]): ParsedRequest | ParseErrorReason {
  let method: string | undefined
  let url: string | undefined
  let raw: string | undefined
  const entries: HeaderEntry[] = []
  const fields: [string, string][] = []
  const query: string[] = []

  const cursor = { i: 0 }
  for (cursor.i = 0; cursor.i < tokens.length; cursor.i++) {
    const token = tokens[cursor.i]
    const [flag, inline] = splitFlag(token)
    const take = () => taker(tokens, cursor, inline)

    // Anything dash-led is a flag, including the `--flag=value` form.
    if (token.startsWith('-')) {
      switch (flag) {
        case '--raw':
          raw = take() ?? raw
          break
        case '--auth':
        case '-a': {
          const v = take()
          if (v) {
            const colon = v.indexOf(':')
            entries.push({
              name: 'Authorization',
              value: basicAuth(
                colon === -1 ? v : v.slice(0, colon),
                colon === -1 ? '' : v.slice(colon + 1)
              ),
            })
          }
          break
        }
        case '--bearer': {
          const v = take()
          if (v) entries.push({ name: 'Authorization', value: `Bearer ${v}` })
          break
        }
        default:
          if (VALUE_FLAGS.httpie.has(flag)) take()
          break
      }
      continue
    }

    if (!url && isUrlOperand(token)) {
      url = token
      continue
    }
    if (!method && METHODS.includes(token.toUpperCase())) {
      method = token.toUpperCase()
      continue
    }

    const operator = httpieOperator(token)
    if (!operator || operator.at === 0) return 'leftover'
    const name = token.slice(0, operator.at)
    const value = token.slice(operator.at + operator.op.length)

    if (operator.op === ':=' || operator.op === '@') return 'unsupported-field'
    if (operator.op === '==') {
      query.push(`${name}=${encodeURIComponent(value)}`)
      continue
    }
    if (operator.op === ':') {
      entries.push({ name, value })
      continue
    }
    fields.push([name, value])
  }

  if (!url) return 'no-url'

  let body = raw ?? ''
  if (!body && fields.length) {
    body = JSON.stringify(Object.fromEntries(fields))
    if (!entries.some((e) => e.name.toLowerCase() === 'content-type')) {
      entries.push({ name: 'Content-Type', value: 'application/json' })
    }
  }

  return {
    method: method ?? (body ? 'POST' : 'GET'),
    url: query.length ? appendQuery(url, query.join('&')) : url,
    requestHeaders: foldHeaders(entries),
    requestBody: body,
  }
}

function splitTopLevel(text: string, sep: string, d: Dialect): string[] {
  const out: string[] = []
  let depth = 0
  let start = 0
  let i = 0
  while (i < text.length) {
    const c = text[i]
    if (c === "'" || c === '"') {
      const r = c === "'" ? readSingle(text, i, d) : readDouble(text, i, d)
      i = r ? r.next : i + 1
      continue
    }
    if (c === '(' || c === '[' || c === '{') depth++
    else if (c === ')' || c === ']' || c === '}') depth--
    else if (c === sep && depth === 0) {
      out.push(text.slice(start, i))
      start = i + 1
    }
    i++
  }
  out.push(text.slice(start))
  return out
}

function psValue(raw: string): string | null {
  const t = raw.trim()
  if (!t) return ''
  if (t.startsWith('$')) return null
  if (t.startsWith("'") || t.startsWith('"')) {
    const r = t.startsWith("'") ? readSingle(t, 0, POWERSHELL) : readDouble(t, 0, POWERSHELL)
    return r ? r.text : null
  }
  return t
}

function psHashtable(text: string): HeaderEntry[] | null {
  const out: HeaderEntry[] = []
  for (const entry of splitTopLevel(text.slice(2, -1), ';', POWERSHELL)) {
    if (!entry.trim()) continue
    const parts = splitTopLevel(entry, '=', POWERSHELL)
    const name = psValue(parts[0])
    const value = psValue(parts.slice(1).join('='))
    if (name === null || value === null || !name) return null
    out.push({ name, value })
  }
  return out
}

function fromPowerShell(tokens: string[]): ParsedRequest | ParseErrorReason {
  let method: string | undefined
  let url: string | undefined
  let body = ''
  const entries: HeaderEntry[] = []

  const cursor = { i: 0 }
  for (cursor.i = 0; cursor.i < tokens.length; cursor.i++) {
    const token = tokens[cursor.i]
    if (!token.startsWith('-')) {
      if (!url && looksLikeUrl(token)) {
        url = token
        continue
      }
      return 'leftover'
    }
    const [flag, inline] = splitFlag(token)
    const take = () => taker(tokens, cursor, inline)

    switch (flag.toLowerCase()) {
      case '-method':
        method = take()?.toUpperCase()
        break
      case '-uri':
        url = take() ?? url
        break
      case '-body': {
        const v = take()
        if (v === null) return 'unsupported-variable'
        body = v
        break
      }
      case '-headers': {
        const v = take()
        if (v === null || v.startsWith('$')) return 'unsupported-variable'
        if (v.startsWith('@{')) {
          const parsed = psHashtable(v)
          if (!parsed) return 'unsupported-variable'
          entries.push(...parsed)
        }
        break
      }
      case '-contenttype': {
        const v = take()
        if (v !== null) entries.push({ name: 'Content-Type', value: v })
        break
      }
      default:
        if (VALUE_FLAGS.powershell.has(flag) || VALUE_FLAGS.powershell.has(flag.toLowerCase())) {
          const v = take()
          if (v !== null && v.startsWith('$')) return 'unsupported-variable'
        }
        break
    }
  }

  if (!url) return 'no-url'

  return {
    method: method ?? (body ? 'POST' : 'GET'),
    url,
    requestHeaders: foldHeaders(withFormType(entries, body !== '')),
    requestBody: body,
  }
}

function matchBracket(text: string, open: number, d: Dialect): number | null {
  let depth = 0
  let i = open
  while (i < text.length) {
    const c = text[i]
    if (c === "'" || c === '"' || c === '`') {
      if (c === '`') {
        let j = i + 1
        while (j < text.length && text[j] !== '`') j += text[j] === '\\' ? 2 : 1
        if (j >= text.length) return null
        i = j + 1
        continue
      }
      const r = c === "'" ? readSingle(text, i, d) : readDouble(text, i, d)
      if (!r) return null
      i = r.next
      continue
    }
    if (c === '(' || c === '[' || c === '{') depth++
    else if (c === ')' || c === ']' || c === '}') {
      depth--
      if (depth === 0) return i + 1
    }
    i++
  }
  return null
}

function jsString(raw: string): string | null {
  const t = raw.trim()
  if (!t) return null
  if (t.startsWith("'")) {
    const r = readSingle(t, 0, POSIX)
    return r && r.next === t.length ? r.text : null
  }
  if (t.startsWith('"')) {
    const r = readDouble(t, 0, POSIX)
    return r && r.next === t.length ? r.text : null
  }
  return null
}

function fromFetch(text: string): ParsedRequest | ParseErrorReason {
  const call = /\bfetch\s*\(/.exec(text)
  if (!call) return 'bad-fetch-init'
  const open = call.index + call[0].length - 1
  const close = matchBracket(text, open, POSIX)
  if (close === null) return 'bad-fetch-init'

  const args = splitTopLevel(text.slice(open + 1, close - 1), ',', POSIX)
  const url = jsString(args[0] ?? '')
  if (url === null) return 'bad-fetch-init'

  if (args.length < 2 || !args[1].trim()) {
    return { method: 'GET', url, requestHeaders: {}, requestBody: '' }
  }

  let init: Record<string, unknown>
  try {
    init = JSON.parse(args[1]) as Record<string, unknown>
  } catch {
    // `fetch(url, options)` — an identifier we cannot resolve at all.
    return 'bad-fetch-init'
  }

  const entries: HeaderEntry[] = []
  const h = init.headers
  if (Array.isArray(h)) {
    for (const pair of h) {
      if (Array.isArray(pair) && pair.length >= 2) {
        entries.push({ name: String(pair[0]), value: String(pair[1]) })
      }
    }
  } else if (h && typeof h === 'object') {
    for (const [name, value] of Object.entries(h as Record<string, unknown>)) {
      entries.push({ name, value: value == null ? '' : String(value) })
    }
  }

  let body = ''
  const b = init.body
  if (typeof b === 'string') body = b
  else if (typeof b === 'number' || typeof b === 'boolean') body = String(b)
  else if (b && typeof b === 'object') body = JSON.stringify(b)

  return {
    method: typeof init.method === 'string' && init.method ? init.method.toUpperCase() : 'GET',
    url,
    requestHeaders: foldHeaders(entries),
    requestBody: body,
  }
}

interface ParseContext {
  tokens: string[]
  text: string
}

type Parser = (ctx: ParseContext) => ParsedRequest | ParseErrorReason

const PARSERS: Record<ExportFormat, Parser> = {
  curl: ({ tokens }) => fromCurl(tokens),
  wget: ({ tokens }) => fromWget(tokens),
  httpie: ({ tokens }) => fromHttpie(tokens),
  powershell: ({ tokens }) => fromPowerShell(tokens),
  fetch: ({ text }) => fromFetch(text),
}

const FETCH_PREFIX =
  /^(?:(?:await|return)\s+|(?:const|let|var)\s+[\w$]+\s*=\s*(?:await\s+)?)*fetch\s*\(/

/** The first token that is an operand rather than a flag or a flag's value. */
function firstOperand(tokens: string[], valueFlags: Set<string>): string {
  for (let i = 0; i < tokens.length; i++) {
    const token = tokens[i]
    if (!token.startsWith('-')) return token
    const [flag, inline] = splitFlag(token)
    if (inline === null && valueFlags.has(flag)) i++
  }
  return ''
}

function detect(text: string): { format: ExportFormat; rest: string; dialect: Dialect } | null {
  const trimmed = text.trim()

  if (/\bfetch\s*\(/.test(trimmed) && FETCH_PREFIX.test(trimmed)) {
    return { format: 'fetch', rest: trimmed, dialect: POSIX }
  }

  const m = /^([^\s;]+)\s*([\s\S]*)$/.exec(trimmed)
  if (!m) return null
  const name = m[1].toLowerCase()
  const rest = m[2]

  if (name === 'curl' || name === 'curl.exe') return { format: 'curl', rest, dialect: SHELL }
  if (name === 'wget' || name === 'wget.exe') return { format: 'wget', rest, dialect: SHELL }
  if (name.startsWith('invoke-') || name === 'irm' || name === 'iwr') {
    return { format: 'powershell', rest, dialect: POWERSHELL }
  }
  if (name === 'http' || name === 'https' || name === 'http.exe') {
    const tokens = tokenize(rest, POSIX)
    if (!tokens) return null
    const operand = firstOperand(tokens, VALUE_FLAGS.httpie)
    if (!METHODS.includes(operand.toUpperCase()) && !isUrlOperand(operand)) return null
    return { format: 'httpie', rest, dialect: POSIX }
  }
  return null
}

export function parseRequestCommand(text: string): ParseResult {
  const normalized = normalizeInput(text)
  if (!normalized.trim()) return { kind: 'none' }

  const found = detect(normalized)
  if (!found) return { kind: 'none' }

  const tokens = found.format === 'fetch' ? [] : tokenize(found.rest, found.dialect)
  if (tokens === null) return { kind: 'error', reason: 'bad-quotes' }

  const parsed = PARSERS[found.format]({ tokens, text: found.rest })
  if (typeof parsed === 'string') return { kind: 'error', reason: parsed }
  return { kind: 'ok', format: found.format, request: parsed }
}
