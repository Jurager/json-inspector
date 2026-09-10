// Generates copy-paste representations of an HTTP request for various tools.

export type ExportFormat = 'curl' | 'fetch' | 'wget' | 'httpie' | 'powershell'

function quote(s: string): string {
  return `'${s.replace(/'/g, `'\\''`)}'`
}

export function exportRequest(
  format: ExportFormat,
  method: string,
  url: string,
  headers: Record<string, string>,
  body: string
): string {
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

// copyToClipboard writes text to the clipboard, falling back to a temporary
// textarea + execCommand when the async Clipboard API is unavailable.
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
