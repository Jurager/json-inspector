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

// Every time in the app is microseconds, because a millisecond is too coarse to say anything about a
// warm connection: its phases are over before the second millisecond ticks, and rounding them to zero
// made a measured request look like one that failed to be measured. A phase under a millisecond gets
// a decimal for the same reason.
export function formatMicros(us: number): string {
  if (us < 1000) return `${(us / 1000).toFixed(1)} мс`
  if (us < 1_000_000) return `${Math.round(us / 1000)} мс`
  return `${(us / 1_000_000).toFixed(2)} с`
}

// The three status bands the badges are coloured by, as a class name.
export function statusBadgeClass(status: number): string {
  if (status >= 200 && status < 300) return 'badge-status-2xx'
  if (status >= 300 && status < 400) return 'badge-status-3xx'
  return 'badge-status-4xx'
}

export function formatVersion(tag: string): string {
  return tag.replace(/^v/, '')
}

const sameDay = (a: Date, b: Date) => a.toDateString() === b.toDateString()

export function formatCheckedAt(ms: number): string {
  if (!ms) return '—'
  const at = new Date(ms)
  const time = at.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })
  const now = new Date()
  if (sameDay(at, now)) return `сегодня в ${time}`
  const yesterday = new Date(now)
  yesterday.setDate(now.getDate() - 1)
  if (sameDay(at, yesterday)) return `вчера в ${time}`
  return `${at.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' })}, ${time}`
}
