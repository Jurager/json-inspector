// Go's map[string]string reaches TypeScript as `{ [key: string]?: string } | null`:
// the v3 binding generator marks map values optional and the map itself
// nullable. Records store headers as Record<string, string>, so normalise once
// at the boundary — a header with no value is dropped rather than being turned
// into the literal string "undefined".
export function normalizeHeaders(
  headers: { [key: string]: string | undefined } | null | undefined
): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [key, value] of Object.entries(headers ?? {})) {
    if (value !== undefined) out[key] = value
  }
  return out
}
