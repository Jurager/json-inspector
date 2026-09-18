// What a row shows of an address: the host and everything after it. The scheme is dropped — it is the
// longest part of a URL and the one that says the least — and the text is cut rather than parsed: an
// address here can be `{{host}}/articles`, or carry a variable that resolved to nothing, and the
// standard parser rewrites the braces of a `{{token}}` into `%7B%7B`, which is not what the user
// typed. The whole address is in the row's title.
export function addressOf(url: string): string {
  return url.replace(/^[a-zA-Z][\w+.-]*:\/\//, '')
}

// The same address in the two inks a row draws it with: the host says where, and everything after it
// says what. Split by the first slash rather than parsed, for the reason above — a `{{token}}` is not
// something to hand to a URL parser. An address with no path has none, rather than a bare slash.
export function splitAddress(url: string): { host: string; path: string } {
  const rest = addressOf(url)
  const cut = rest.indexOf('/')
  return cut < 0 ? { host: rest, path: '' } : { host: rest.slice(0, cut), path: rest.slice(cut) }
}
