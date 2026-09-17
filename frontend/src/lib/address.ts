// What a row shows of an address: the host and everything after it. The scheme is dropped — it is the
// longest part of a URL and the one that says the least — and the text is cut rather than parsed: an
// address here can be `{{host}}/articles`, or carry a variable that resolved to nothing, and the
// standard parser rewrites the braces of a `{{token}}` into `%7B%7B`, which is not what the user
// typed. The whole address is in the row's title.
export function addressOf(url: string): string {
  return url.replace(/^[a-zA-Z][\w+.-]*:\/\//, '')
}
