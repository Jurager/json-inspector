// Dumps the current TypeScript parser's behaviour as fixtures for the Go port.
//
// Run from frontend/:  node tools/dump-fixtures.mjs
//
// The port is only correct if it reproduces these files, so they are generated from the TS
// implementation *before* it is deleted and committed as the oracle. The inputs are written as raw
// bytes in their own files: they contain backticks, $'…', CRLF, a BOM and shell redirections, and
// a Go string literal would silently change some of them.
import { mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createServer } from 'vite'

const here = dirname(fileURLToPath(import.meta.url))
const outRoot = join(here, '..', '..', 'internal', 'command', 'testdata')
const parseDir = join(outRoot, 'parse')
const exportDir = join(outRoot, 'export')

// Every case the parser has to keep behaving the same way for. Grouped by what they probe, not by
// format, because that is how a port drifts: the quoting rules are shared across dialects.
const PARSE_CASES = [
  // — curl: the happy paths and the flags that change what is sent —
  ['curl_basic', `curl 'https://api.example.com/articles'`],
  ['curl_no_quotes', `curl https://api.example.com/articles`],
  ['curl_method', `curl -X POST https://api.example.com/articles`],
  ['curl_method_long', `curl --request DELETE https://api.example.com/articles/1`],
  ['curl_header', `curl -H 'Accept: application/vnd.api+json' https://api.example.com/articles`],
  ['curl_header_long', `curl --header "X-Request-Id: abc-123" https://api.example.com/articles`],
  ['curl_two_headers', `curl -H 'A: 1' -H 'B: 2' https://api.example.com/articles`],
  ['curl_data_raw', `curl -X POST --data-raw '{"a":1}' https://api.example.com/articles`],
  ['curl_data', `curl -X POST -d 'a=1&b=2' https://api.example.com/articles`],
  ['curl_data_at_file', `curl -X POST --data @payload.json https://api.example.com/articles`],
  ['curl_json', `curl --json '{"a":1}' https://api.example.com/articles`],
  ['curl_get_with_data', `curl -G -d 'page[number]=2' https://api.example.com/articles`],
  ['curl_user', `curl -u user:secret https://api.example.com/articles`],
  ['curl_cookie', `curl -b 'session=abc' https://api.example.com/articles`],
  ['curl_url_flag', `curl --url 'https://api.example.com/articles'`],
  ['curl_url_flag_before_positional', `curl --url 'https://a.example.com/x' 'https://b.example.com/y'`],
  ['curl_flag_equals', `curl --request=POST https://api.example.com/articles`],
  ['curl_double_dash', `curl -- 'https://api.example.com/articles'`],
  ['curl_before_url_value_flags', `curl -H 'A: 1' --data-raw '{}' https://api.example.com/articles`],
  ['curl_multipart', `curl -F 'file=@photo.png' https://api.example.com/upload`],
  ['curl_output_flag', `curl -o out.json https://api.example.com/articles`],
  ['curl_unknown_flag', `curl --compressed https://api.example.com/articles`],
  ['curl_no_url', `curl -X POST`],
  ['curl_leftover', `curl https://api.example.com/articles extra.txt`],
  ['curl_redirect', `curl https://api.example.com/articles 2>&1`],
  ['curl_sudo', `sudo curl https://api.example.com/articles`],
  ['curl_double_quoted_url', `curl "https://api.example.com/articles?page[size]=10"`],
  ['curl_single_quoted_with_quote', `curl -H 'X-Q: it'\\''s' https://api.example.com/articles`],
  ['curl_ansi_c', `curl -H $'X-Tab:\\tvalue' https://api.example.com/articles`],
  ['curl_line_continuation', `curl -H 'A: 1' \\\n  https://api.example.com/articles`],
  ['curl_crlf', `curl -H 'A: 1' https://api.example.com/articles\r\n`],
  ['curl_bom', `\uFEFFcurl https://api.example.com/articles`],
  ['curl_windows_prompt', `PS C:\\Users\\dev> curl https://api.example.com/articles`],
  ['curl_tokens', `curl -H 'Authorization: Bearer {{token}}' '{{base_url}}/articles'`],
  ['curl_empty_quoting', `curl -H '' https://api.example.com/articles`],

  // — wget —
  ['wget_basic', `wget https://api.example.com/articles`],
  ['wget_method', `wget --method=POST --body-data='{"a":1}' https://api.example.com/articles`],
  ['wget_header', `wget --header='Accept: application/json' https://api.example.com/articles`],
  ['wget_output', `wget -O - https://api.example.com/articles`],

  // — httpie —
  ['httpie_get', `http GET https://api.example.com/articles`],
  ['httpie_post_json', `http POST https://api.example.com/articles name=test`],
  ['httpie_raw_json', `http --raw '{"a":1}' POST https://api.example.com/articles`],
  ['httpie_header', `http GET https://api.example.com/articles Accept:application/json`],
  ['httpie_json_field', `http POST https://api.example.com/articles count:=3`],
  ['httpie_query', `http GET https://api.example.com/articles page==2`],
  ['httpie_file_field', `http POST https://api.example.com/articles file@photo.png`],

  // — PowerShell —
  ['ps_irm_basic', `Invoke-RestMethod -Uri 'https://api.example.com/articles'`],
  ['ps_irm_method_body', `Invoke-RestMethod -Method Post -Uri 'https://api.example.com/articles' -Body '{"a":1}'`],
  ['ps_irm_headers', `Invoke-RestMethod -Uri 'https://api.example.com/articles' -Headers @{ 'Accept' = 'application/json'; 'X-Request-Id' = 'abc' }`],
  ['ps_alias_iwr', `iwr -Uri 'https://api.example.com/articles'`],
  ['ps_short_alias_irm', `irm 'https://api.example.com/articles'`],
  ['ps_double_quoted_escape', `Invoke-RestMethod -Uri "https://api.example.com/o''brien"`],
  ['ps_line_continuation', "Invoke-RestMethod `\n  -Uri 'https://api.example.com/articles'"],
  ['ps_variable', `Invoke-RestMethod -Uri $baseUrl -Method Post`],

  // — fetch: the init has to be strict JSON, which is what devtools' "Copy as fetch" emits —
  ['fetch_basic', `fetch('https://api.example.com/articles')`],
  ['fetch_devtools_get', `fetch("https://api.example.com/articles", {"headers": {"accept": "application/vnd.api+json"}})`],
  ['fetch_devtools_post', `fetch("https://api.example.com/articles", {"headers": {"content-type": "application/json"}, "body": "{\\"data\\":{\\"type\\":\\"articles\\"}}", "method": "POST"})`],
  ['fetch_method_only', `fetch('https://api.example.com/articles', {"method":"POST"})`],
  ['fetch_headers_array', `fetch('https://api.example.com/articles', {"method":"POST","headers":[["X-A","1"],["X-B","2"]]})`],
  ['fetch_body_object', `fetch('https://api.example.com/articles', {"method":"POST","body":{"a":1}})`],
  ['fetch_body_null_header', `fetch('https://api.example.com/articles', {"headers":{"X-Empty":null}})`],
  ['fetch_const_prefix', `const res = await fetch('https://api.example.com/articles', {"method":"GET"})`],
  // JS object literals are not JSON, so they cannot be read — pinned on purpose.
  ['fetch_js_literal_init', `fetch('https://api.example.com/articles', { method: 'POST' })`],
  ['fetch_body_stringify', `fetch('https://api.example.com/articles', { method: 'POST', body: JSON.stringify({ a: 1 }) })`],
  ['fetch_non_literal', `fetch(url, { method: 'POST' })`],

  // — what must NOT be taken for a command —
  ['none_empty', ``],
  ['none_whitespace', `   \n  `],
  ['none_plain_url', `https://api.example.com/articles`],
  ['none_json', `{"a": 1}`],
  ['none_prose', `сегодня надо проверить эндпоинт статей`],
  ['none_curl_word', `curl`],
  ['none_unclosed_quote', `curl 'https://api.example.com/articles`],
]

// Export cases: the parsed result of a few representative commands, rendered in each format. The
// round trip parse → export → parse is asserted separately, in Go, over the parse fixtures.
const EXPORT_REQUESTS = [
  {
    name: 'get_simple',
    method: 'GET',
    url: 'https://api.example.com/articles',
    headers: [],
    body: '',
  },
  {
    name: 'get_query_and_headers',
    method: 'GET',
    url: 'https://api.example.com/articles?page[size]=10&include=author',
    headers: [
      ['Accept', 'application/vnd.api+json'],
      ['X-Request-Id', 'abc-123'],
    ],
    body: '',
  },
  {
    name: 'post_json_body',
    method: 'POST',
    url: 'https://api.example.com/articles',
    headers: [['Content-Type', 'application/json']],
    body: '{"data":{"type":"articles","attributes":{"title":"Hi"}}}',
  },
  {
    name: 'tokens_and_quotes',
    method: 'POST',
    url: '{{base_url}}/articles',
    headers: [['Authorization', 'Bearer {{token}}']],
    body: `{"title":"it's a 'quoted' one"}`,
  },
]

const FORMATS = ['curl', 'wget', 'httpie', 'powershell', 'fetch']

const server = await createServer({
  root: join(here, '..'),
  server: { middlewareMode: true },
  appType: 'custom',
  logLevel: 'error',
})

try {
  const { parseRequestCommand } = await server.ssrLoadModule('/src/lib/parseRequest.ts')
  const { exportRequest } = await server.ssrLoadModule('/src/lib/export.ts')

  rmSync(parseDir, { recursive: true, force: true })
  rmSync(exportDir, { recursive: true, force: true })
  mkdirSync(parseDir, { recursive: true })
  mkdirSync(exportDir, { recursive: true })

  const counts = { none: 0, ok: 0, error: 0 }
  const errors = []

  PARSE_CASES.forEach(([name, input], index) => {
    const slug = `${String(index + 1).padStart(3, '0')}_${name}`
    const result = parseRequestCommand(input)
    counts[result.kind] += 1

    writeFileSync(join(parseDir, `${slug}.cmd`), input)
    const want = { kind: result.kind }
    if (result.kind === 'ok') {
      want.format = result.format
      want.method = result.request.method
      want.url = result.request.url
      want.headers = Object.entries(result.request.requestHeaders)
      want.body = result.request.requestBody
    }
    if (result.kind === 'error') want.reason = result.reason
    writeFileSync(join(parseDir, `${slug}.want.json`), JSON.stringify(want, null, 2) + '\n')
    if (result.kind === 'error') errors.push(`${slug}: ${result.reason}`)
  })

  let exports = 0
  for (const request of EXPORT_REQUESTS) {
    for (const format of FORMATS) {
      const slug = `${request.name}_${format}`
      const headers = Object.fromEntries(request.headers)
      const input = {
        format,
        method: request.method,
        url: request.url,
        headers: request.headers,
        body: request.body,
      }
      const want = exportRequest(format, {
        method: request.method,
        url: request.url,
        requestHeaders: headers,
        requestBody: request.body,
      })
      writeFileSync(join(exportDir, `${slug}.in.json`), JSON.stringify(input, null, 2) + '\n')
      writeFileSync(join(exportDir, `${slug}.want.txt`), want)
      exports += 1
    }
  }

  console.log(`parse: ${PARSE_CASES.length} cases (ok ${counts.ok}, none ${counts.none}, error ${counts.error})`)
  if (errors.length) console.log('errors:', errors.join(', '))
  console.log(`export: ${exports} cases`)
} finally {
  await server.close()
}
