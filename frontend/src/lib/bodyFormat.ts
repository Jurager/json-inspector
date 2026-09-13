import { BodyKind } from '../../bindings/json-inspector/internal/domain'
import { prettyJson, tryParseJson } from './json'

// Tidying up what is being typed. It is the window's own business and not Go's: this is a text edit
// the user could have made by hand, like typing, and it only ever replaces the text with a re-typed
// version of itself.
//
// A text that does not parse is handed back untouched. Reformatting is a convenience, and one that
// quietly mangled a body it did not understand would be worse than not offering it.

export function formatBody(kind: BodyKind, text: string): string {
  if (text.trim() === '') return text
  if (kind === BodyKind.BodyJSON) {
    const parsed = tryParseJson(text)
    return parsed.ok ? prettyJson(parsed.value) : text
  }
  if (kind === BodyKind.BodyXML) {
    return formatXML(text)
  }
  return text
}

// formatXML re-indents a document that has tags to indent. It is not a parser: a text whose tags do
// not balance comes back with lines the reader can still see, because the XML editor is a text box
// and not a validator.
//
// A line that opens and closes one element — <name>Кофемолка</name> — is written on one line, which
// is what the design draws and what a reader wants: XML nests, it does not stack.
function formatXML(text: string): string {
  const pieces = text
    .trim()
    // Whitespace between two tags is formatting, and this function is about to choose its own.
    .replace(/>\s+</g, '><')
    .replace(/></g, '>\n<')
    .split('\n')

  const lines: string[] = []
  let depth = 0
  for (const piece of pieces) {
    const line = piece.trim()
    if (line === '') continue

    if (line.startsWith('</')) depth = Math.max(0, depth - 1)
    lines.push('  '.repeat(depth) + line)

    // A bare opening tag steps in; a closing one has already stepped out; a self-closing one and a
    // one-line element change nothing.
    const opens =
      line.startsWith('<') && !line.startsWith('</') && !line.startsWith('<?') &&
      !line.endsWith('/>') && !/<\/[^>]+>$/.test(line) && !line.slice(1).startsWith('!')
    if (opens) depth += 1
  }
  return lines.join('\n')
}
