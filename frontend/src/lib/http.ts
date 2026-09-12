// Go's map[string]string reaches TypeScript as `{ [key: string]?: string } | null`
// (the v3 binding marks map values optional and the map nullable). A header with
// no value is dropped here, never turned into the literal string "undefined".
export function normalizeHeaders(
  headers: { [key: string]: string | undefined } | null | undefined
): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [key, value] of Object.entries(headers ?? {})) {
    if (value !== undefined) out[key] = value
  }
  return out
}
