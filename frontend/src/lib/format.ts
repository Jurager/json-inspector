// What is left here is the formatting that has no language in it. Everything that depends on the
// language — numbers, dates, units — lives in `../i18n`, because that is where the locale is.

// The three status bands the badges are coloured by, as a class name.
export function statusBadgeClass(status: number): string {
  if (status >= 200 && status < 300) return 'badge-status-2xx'
  if (status >= 300 && status < 400) return 'badge-status-3xx'
  return 'badge-status-4xx'
}

// The colour a verb reads in: GET fetches, DELETE destroys, and everything else writes. The design
// gives the three one class each, and every list draws its verb with this — the same word has to look
// the same in the history, in a collection and in a table of results.
export function methodInkClass(method: string): string {
  switch (method.toUpperCase()) {
    case 'GET':
      return 'method-get'
    case 'DELETE':
      return 'method-delete'
    default:
      return 'method-other'
  }
}

// A verb as the narrow column can hold it: the two long ones are cut the way the design cuts them,
// because DELETE at eleven points is wider than the column the names line up in.
export function shortMethod(method: string): string {
  switch (method.toUpperCase()) {
    case 'DELETE':
      return 'DEL'
    case 'OPTIONS':
      return 'OPT'
    default:
      return method
  }
}

export function formatVersion(tag: string): string {
  return tag.replace(/^v/, '')
}
