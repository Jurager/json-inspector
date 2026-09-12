// Go's map[string]string arrives as `{ [key: string]?: string } | null`: the v3 binding marks
// values optional and the map nullable. A valueless header is dropped, not stringified.
export function normalizeHeaders(
  headers: { [key: string]: string | undefined } | null | undefined
): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [key, value] of Object.entries(headers ?? {})) {
    if (value !== undefined) out[key] = value
  }
  return out
}
