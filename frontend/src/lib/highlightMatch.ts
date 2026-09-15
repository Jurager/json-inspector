const ESCAPES: Record<string, string> = {
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
  "'": '&#39;',
}

function escapeHtml(text: string): string {
  return text.replace(/[&<>"']/g, (ch) => ESCAPES[ch])
}

/**
 * The text with every occurrence of the query wrapped in a marked span, as HTML.
 *
 * This is not a second search: Go decided that the row answers and this only says where. Matching
 * there is a case-insensitive substring of the same text, so looking for it the same way here finds
 * the same spans — and if it ever finds none, the row is drawn as plain text rather than refused.
 *
 * Both halves are escaped, because the names in this app are the user's and a collection called
 * `<b>` is a collection, not markup.
 */
export function highlightMatch(text: string, query: string): string {
  const needle = query.trim().toLowerCase()
  if (!needle) return escapeHtml(text)

  const haystack = text.toLowerCase()
  let out = ''
  let at = 0
  for (;;) {
    const found = haystack.indexOf(needle, at)
    if (found < 0) break
    out += escapeHtml(text.slice(at, found))
    out += `<b class="search-match">${escapeHtml(text.slice(found, found + needle.length))}</b>`
    at = found + needle.length
  }
  return out + escapeHtml(text.slice(at))
}
