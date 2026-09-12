import { substituteTokensMasked, type ResolveFn } from './vars'
import type { RequestRecord } from './requestRecord'

export type ExportFormat = 'curl' | 'fetch' | 'wget' | 'httpie' | 'powershell'

export interface ExportOptions {
  resolve?: ResolveFn
  keepTokens?: boolean
}

function shellQuote(s: string): string {
  return `'${s.replace(/'/g, `'\\''`)}'`
}

function psQuote(s: string): string {
  return `'${s.replace(/'/g, "''")}'`
}

function substituteForExport(text: string, options?: ExportOptions): string {
  if (!options?.resolve || options.keepTokens) return text
  return substituteTokensMasked(text, options.resolve)
}

export function exportRequest(
  format: ExportFormat,
  request: Pick<RequestRecord, 'method' | 'url' | 'requestHeaders' | 'requestBody'>,
  options?: ExportOptions
): string {
  const url = substituteForExport(request.url, options)
  const body = substituteForExport(request.requestBody, options)
  const headers: Record<string, string> = {}
  for (const [k, v] of Object.entries(request.requestHeaders)) {
    headers[substituteForExport(k, options)] = substituteForExport(v, options)
  }
  const entries = Object.entries(headers)
  const method = request.method
  switch (format) {
    case 'curl': {
      const parts = ['curl']
      if (method !== 'GET') parts.push('-X', method)
      for (const [k, v] of entries) parts.push('-H', shellQuote(`${k}: ${v}`))
      if (body) parts.push('--data-raw', shellQuote(body))
      parts.push(shellQuote(url))
      return parts.join(' ')
    }
    case 'fetch': {
      const init: Record<string, unknown> = { method }
      if (entries.length) init.headers = headers
      if (body) init.body = body
      return `fetch(${shellQuote(url)}, ${JSON.stringify(init)})`
    }
    case 'wget': {
      const parts = ['wget']
      if (method !== 'GET') parts.push(`--method=${method}`)
      for (const [k, v] of entries) parts.push(`--header=${shellQuote(`${k}: ${v}`)}`)
      if (body) parts.push(`--body-data=${shellQuote(body)}`)
      parts.push(shellQuote(url))
      return parts.join(' ')
    }
    case 'httpie': {
      const parts = ['http']
      if (body) parts.push(`--raw=${shellQuote(body)}`)
      parts.push(method, shellQuote(url))
      for (const [k, v] of entries) parts.push(shellQuote(`${k}:${v}`))
      return parts.join(' ')
    }
    case 'powershell': {
      const parts = [`Invoke-RestMethod -Method ${method} -Uri ${psQuote(url)}`]
      if (entries.length) {
        const h = entries.map(([k, v]) => `${psQuote(k)} = ${psQuote(v)}`).join('; ')
        parts.push(`-Headers @{ ${h} }`)
      }
      if (body) parts.push(`-Body ${psQuote(body)}`)
      return parts.join(' ')
    }
  }
}
