import type { CookieRow as WireCookieRow } from '../../bindings/json-inspector/internal/domain'

// A request's own `Cookie` header — the jar the "Cookies" tab edits in "Запрос" mode. Every
// attribute is present here, because a form field cannot bind to the ones the wire leaves out when
// they are empty.
export type CookieRow = Required<WireCookieRow>

export function emptyCookieRow(): CookieRow {
  return { name: '', value: '', domain: '', path: '/', expires: '', secure: false, httpOnly: false }
}

// A row out of a stored record, with the attributes the editor binds to filled in.
export function cookieRowFrom(row: WireCookieRow): CookieRow {
  return { ...emptyCookieRow(), ...row }
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
      return { ...emptyCookieRow(), name, value }
    })
    .filter((c) => c.name)
}
