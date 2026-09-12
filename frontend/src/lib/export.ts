import { parseTokens, SECRET_MASK, type ResolveFn } from './vars'

export type ExportFormat = 'curl' | 'fetch' | 'wget' | 'httpie' | 'powershell'

export interface ExportOptions {
  resolve?: ResolveFn
  keepTokens?: boolean
}

function quote(s: string): string {
  return `'${s.replace(/'/g, `'\\''`)}'`
}

// Like the send path, but a secret never comes out: an export is shared, pasted
// and screenshotted, so it carries the dots.
function render(text: string, opts?: ExportOptions): string {
  if (!opts?.resolve || opts.keepTokens) return text
  const tokens = parseTokens(text)
  if (tokens.length === 0) return text
  let out = ''
  let last = 0
  for (const t of tokens) {
    const r = opts.resolve(t.name)
    out += text.slice(last, t.start)
    out += r ? (r.kind === 'secret' ? SECRET_MASK : r.value) : t.raw
    last = t.end
  }
  return out + text.slice(last)
}

export function exportRequest(
  format: ExportFormat,
  method: string,
  url: string,
  headers: Record<string, string>,
  body: string,
  opts?: ExportOptions
): string {
  url = render(url, opts)
  body = render(body, opts)
  const resolved: Record<string, string> = {}
  for (const [k, v] of Object.entries(headers)) resolved[render(k, opts)] = render(v, opts)
  headers = resolved
  const entries = Object.entries(headers)
  switch (format) {
    case 'curl': {
      const parts = ['curl']
      if (method !== 'GET') parts.push('-X', method)
      for (const [k, v] of entries) parts.push('-H', quote(`${k}: ${v}`))
      if (body) parts.push('--data-raw', quote(body))
      parts.push(quote(url))
      return parts.join(' ')
    }
    case 'fetch': {
      const init: Record<string, unknown> = { method }
      if (entries.length) init.headers = headers
      if (body) init.body = body
      return `fetch(${quote(url)}, ${JSON.stringify(init)})`
    }
    case 'wget': {
      const parts = ['wget']
      if (method !== 'GET') parts.push(`--method=${method}`)
      for (const [k, v] of entries) parts.push(`--header=${quote(`${k}: ${v}`)}`)
      if (body) parts.push(`--body-data=${quote(body)}`)
      parts.push(quote(url))
      return parts.join(' ')
    }
    case 'httpie': {
      const parts = ['http']
      if (body) parts.push(`--raw=${quote(body)}`)
      parts.push(method, quote(url))
      for (const [k, v] of entries) parts.push(quote(`${k}:${v}`))
      return parts.join(' ')
    }
    case 'powershell': {
      const parts = [`Invoke-RestMethod -Method ${method} -Uri ${quote(url)}`]
      if (entries.length) {
        const h = entries.map(([k, v]) => `${k} = ${quote(v)}`).join('; ')
        parts.push(`-Headers @{ ${h} }`)
      }
      if (body) parts.push(`-Body ${quote(body)}`)
      return parts.join(' ')
    }
  }
}

export async function copyToClipboard(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    try {
      const ta = document.createElement('textarea')
      ta.value = text
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      const ok = document.execCommand('copy')
      document.body.removeChild(ta)
      return ok
    } catch {
      return false
    }
  }
}
