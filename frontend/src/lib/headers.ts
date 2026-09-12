export function normalizeHeaders(
  headers: { [key: string]: string | undefined } | null | undefined
): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [key, value] of Object.entries(headers ?? {})) {
    if (value !== undefined) out[key] = value
  }
  return out
}
