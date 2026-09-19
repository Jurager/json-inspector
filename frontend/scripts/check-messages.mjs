// Checks that every message key the code asks for is in the catalogue, and that both catalogues have
// the same shape.
//
// Why this exists: vue-i18n's typed keys do not reach the composer's `t` in this setup — a key that
// does not exist type-checks and then renders as its own name at runtime, which is a screen with
// `request.chips.bodd` written on it rather than a build failure. `satisfies Messages` in the Russian
// catalogue catches a translation that was forgotten; nothing catches a call site that was invented.
// Run it the way `vue-tsc` is run — see `npm run build`.
//
// The catalogues are imported as TypeScript, which needs Node's type stripping (23.6 and up).
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, dirname } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const SRC = join(root, 'src')

function keysOf(node, prefix = '') {
  return Object.entries(node).flatMap(([key, value]) =>
    value && typeof value === 'object' && !Array.isArray(value)
      ? keysOf(value, `${prefix}${key}.`)
      : [`${prefix}${key}`]
  )
}

async function catalogue(name) {
  const url = pathToFileURL(join(SRC, 'locales', `${name}.ts`)).href
  return new Set(keysOf((await import(url)).default))
}

function files(dir) {
  return readdirSync(dir).flatMap((entry) => {
    const path = join(dir, entry)
    if (statSync(path).isDirectory()) return entry === 'locales' ? [] : files(path)
    return /\.(vue|ts)$/.test(entry) ? [path] : []
  })
}

// `t('a.b')` and `t(\`a.b\`)`. A key the code builds from a variable — `t(\`chips.${name}\`)` — is
// cut at the substitution, and only its fixed prefix can be checked, which is the strongest thing that
// can be said about it without running the app.
const CALL = /\bt\(\s*(['"`])([^'"`]*)\1/g

// A key also reaches `t` through a helper, where the scan above cannot see it: `asked(call, key)` in
// the stores hands its second argument to `t` on the refusal path. Those keys are written by hand the
// same way, and one of them went to a catalogue that never had it — the window then showed the key's
// own name in a toast, and this script said nothing.
//
// The call is walked to its own closing bracket rather than matched by a pattern: the first argument
// is usually a call of its own with commas and brackets in it, and a pattern that stopped at the first
// comma would quietly skip the very sites this is here to check.
function askedKeys(source) {
  const keys = []
  for (const call of source.matchAll(/\basked\(/g)) {
    const start = call.index + call[0].length
    let depth = 1
    let end = start
    for (; end < source.length && depth > 0; end++) {
      if (source[end] === '(') depth++
      else if (source[end] === ')') depth--
    }
    const args = source.slice(start, end - 1)
    const last = [...args.matchAll(/(['"`])([^'"`]*)\1/g)].pop()
    // Nothing may follow it: a literal in the middle is something the call was given, not its key.
    if (last && args.slice(last.index + last[0].length).trim() === '') keys.push(last[2])
  }
  return keys
}

const en = await catalogue('en')
const ru = await catalogue('ru')
const problems = []

for (const key of en) if (!ru.has(key)) problems.push(`en has ${key}, ru does not`)
for (const key of ru) if (!en.has(key)) problems.push(`ru has ${key}, en does not`)

// Every code Go can refuse with needs a sentence: a code the catalogue does not know is a failure the
// window shows as the machine's text, and nothing else would notice.
const GO_CODES = join(root, '..', 'internal', 'domain', 'failure.go')
const declared = [...readFileSync(GO_CODES, 'utf8').matchAll(/Code = "([^"]+)"/g)].map((m) => m[1])
for (const code of declared) {
  if (!en.has(`errors.codes.${code}`)) problems.push(`Go refuses with '${code}', which no catalogue words`)
}

for (const path of files(SRC)) {
  const source = readFileSync(path, 'utf8')
  const used = [...source.matchAll(CALL)].map((m) => m[2]).concat(askedKeys(source))
  for (const raw of used) {
    // A key with no dot names nothing in a nested catalogue.
    const key = raw.split('${')[0]
    if (!key.includes('.')) continue
    const open = raw.includes('${') || key.endsWith('.')
    const known = open ? [...en].some((k) => k.startsWith(key)) : en.has(key)
    if (!known) problems.push(`${path.slice(root.length)}: '${raw}' is not in the catalogue`)
  }
}

if (problems.length) {
  console.error(`messages: ${problems.length} problem(s)`)
  for (const problem of problems) console.error('  ' + problem)
  process.exit(1)
}
console.log(`messages: ${en.size} keys — both catalogues agree, every key used in code exists`)
