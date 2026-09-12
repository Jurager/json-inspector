export interface DotenvEntry {
  name: string
  value: string
  secret: boolean
}

// A name hint only: the user confirms or flips each row in the import dialog.
const SECRET_HINT = /(TOKEN|SECRET|PASSWORD|KEY|AUTH)/i

export function parseDotenv(text: string): DotenvEntry[] {
  const out: DotenvEntry[] = []
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim()
    if (!line || line.startsWith('#')) continue
    const body = line.startsWith('export ') ? line.slice(7).trim() : line
    const eq = body.indexOf('=')
    if (eq === -1) continue
    const name = body.slice(0, eq).trim()
    if (!name) continue
    let value = body.slice(eq + 1).trim()
    const quoted =
      (value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))
    if (quoted && value.length >= 2) value = value.slice(1, -1)
    out.push({ name, value, secret: SECRET_HINT.test(name) })
  }
  return out
}
