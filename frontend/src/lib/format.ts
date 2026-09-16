// What is left here is the formatting that has no language in it. Everything that depends on the
// language — numbers, dates, units — lives in `../i18n`, because that is where the locale is.

// The three status bands the badges are coloured by, as a class name.
export function statusBadgeClass(status: number): string {
  if (status >= 200 && status < 300) return 'badge-status-2xx'
  if (status >= 300 && status < 400) return 'badge-status-3xx'
  return 'badge-status-4xx'
}

export function formatVersion(tag: string): string {
  return tag.replace(/^v/, '')
}
