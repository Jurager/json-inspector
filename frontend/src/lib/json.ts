export interface ParsedJson {
  ok: boolean
  value: unknown
  error?: string
}

export function tryParseJson(text: string): ParsedJson {
  if (text == null || text.trim() === '') {
    return { ok: false, value: null, error: 'Пустое тело ответа' }
  }
  try {
    return { ok: true, value: JSON.parse(text) }
  } catch (e) {
    return { ok: false, value: null, error: e instanceof Error ? e.message : String(e) }
  }
}

export function prettyJson(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2) ?? ''
  } catch {
    return String(value)
  }
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

export function highlightJson(text: string): string {
  const out: string[] = []
  const n = text.length
  let i = 0
  while (i < n) {
    const c = text[i]
    if (c === '"') {
      let j = i + 1
      while (j < n) {
        if (text[j] === '\\') {
          j += 2
          continue
        }
        if (text[j] === '"') break
        j++
      }
      const str = text.slice(i, Math.min(j + 1, n))
      out.push(`<span class="tok-str">${escapeHtml(str)}</span>`)
      i = Math.min(j + 1, n)
    } else if (/^-?\d/.test(text.slice(i))) {
      let j = i
      while (j < n && /[0-9eE+\-.]/.test(text[j])) j++
      out.push(`<span class="tok-num">${escapeHtml(text.slice(i, j))}</span>`)
      i = j
    } else if (text.startsWith('true', i) || text.startsWith('false', i)) {
      const len = text.startsWith('true', i) ? 4 : 5
      out.push(`<span class="tok-bool">${text.slice(i, i + len)}</span>`)
      i += len
    } else if (text.startsWith('null', i)) {
      out.push('<span class="tok-null">null</span>')
      i += 4
    } else {
      out.push(escapeHtml(c))
      i++
    }
  }
  return out.join('')
}

export function formatBytes(n: number): string {
  if (!n) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(v >= 100 || i === 0 ? 0 : 1)} ${units[i]}`
}

export function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms} ms`
  return `${(ms / 1000).toFixed(2)} s`
}

export function statusClass(status: number): string {
  if (status >= 200 && status < 300) return 'badge-status-2xx'
  if (status >= 300 && status < 400) return 'badge-status-3xx'
  return 'badge-status-4xx'
}
