// A request's own `Cookie` header — the jar the "Cookies" tab edits in "Запрос" mode.
export interface CookieRow {
  name: string
  value: string
  domain: string
  path: string
  expires: string
  secure: boolean
  httpOnly: boolean
}

export function emptyCookieRow(): CookieRow {
  return { name: '', value: '', domain: '', path: '/', expires: '', secure: false, httpOnly: false }
}

// What actually goes on the wire is just `name=value` pairs — domain/path/expires/flags are
// Set-Cookie (response) attributes and were never part of a request's Cookie header to begin with.
export function cookieHeaderValue(cookies: CookieRow[]): string {
  return cookies
    .filter((c) => c.name.trim())
    .map((c) => `${c.name.trim()}=${c.value}`)
    .join('; ')
}

// Best-effort recovery of a `Cookie` header typed by hand before this editor existed — only
// name/value survive; the other fields never travelled in a request header in the first place.
export function parseCookieHeader(raw: string): CookieRow[] {
  return raw
    .split(';')
    .map((pair) => {
      const eq = pair.indexOf('=')
      const name = (eq === -1 ? pair : pair.slice(0, eq)).trim()
      const value = eq === -1 ? '' : pair.slice(eq + 1).trim()
      return { name, value, domain: '', path: '/', expires: '', secure: false, httpOnly: false }
    })
    .filter((c) => c.name)
}
