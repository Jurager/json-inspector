// Whether pasted text is worth handing to the reader at all. Only a curl command is read: it is the
// one notation a person copies a request out of.
//
// This is a shape check and not a parser: the reading lives in Go, and it is what decides what the
// text actually is. All this decides is whether the paste is worth interrupting, and it has to
// decide it before the answer could possibly arrive — a browser paste is synchronous, and
// preventDefault has to be called inside the handler. The common paste into an address field is an
// address, and an address is never a command.
//
// Being wrong costs nothing: a text this lets through is pasted by the browser as text, and one it
// intercepts that turns out to be prose comes back as `none` and is put in the field by hand.

// A shell prompt a copied line may still carry, and sudo, which the reader strips before it looks —
// stripped here for the same reason, so that what this tests is the same head the reader sees.
const PROMPT = /^(?:PS[^>\n]*>|>|\$|❯)\s+/
const SUDO = /^sudo\s+/

// The word a command starts with. `curl.exe` counts, which the word boundary already gives, and a
// URL does not — a command word is what the reader requires too, so the two agree on what is worth
// looking at.
const COMMAND = /^curl(?:\.exe)?\b/i

export function looksLikeCommand(text: string): boolean {
  let rest = text.trimStart()
  for (;;) {
    const stripped = rest.replace(PROMPT, '').replace(SUDO, '')
    if (stripped === rest) break
    rest = stripped
  }
  return COMMAND.test(rest)
}
