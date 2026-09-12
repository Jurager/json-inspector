package command

import (
	"regexp"
	"strings"
)

// methods are the words a request line may start with. Anything else at that position is a URL, and
// a word that is neither is what makes `http` prose rather than a command.
var methods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true,
	"DELETE": true, "HEAD": true, "OPTIONS": true,
}

// valueFlags is the per-tool table of flags that take a value. A flag listed here is a flag, so its
// value is not an operand: without the table `curl -o out.json URL` would offer `out.json` as the
// URL, and `http -o out URL` would not even be recognised as an httpie command.
var valueFlags = map[Format]map[string]bool{
	FormatCurl: {
		"-X": true, "--request": true, "-H": true, "--header": true, "-d": true, "--data": true,
		"--data-raw": true, "--data-binary": true, "--data-ascii": true, "--data-urlencode": true,
		"--url": true, "-u": true, "--user": true, "-A": true, "--user-agent": true, "-b": true,
		"--cookie": true, "-c": true, "--cookie-jar": true, "-e": true, "--referer": true,
		"-x": true, "--proxy": true, "-o": true, "--output": true, "-T": true, "--upload-file": true,
		"-F": true, "--form": true, "--form-string": true, "-m": true, "--max-time": true,
		"--connect-timeout": true, "--retry": true, "--resolve": true, "--cert": true, "--key": true,
		"--cacert": true, "--capath": true, "--interface": true, "--limit-rate": true, "-w": true,
		"--write-out": true, "-K": true, "--config": true, "--json": true, "--oauth2-bearer": true,
	},
	FormatWget: {
		"-O": true, "--output-document": true, "-o": true, "--output-file": true, "-P": true,
		"--directory-prefix": true, "--method": true, "--header": true, "--body-data": true,
		"--post-data": true, "--body-file": true, "--timeout": true, "-T": true, "--user": true,
		"--password": true, "--user-agent": true, "-U": true, "--referer": true, "--limit-rate": true,
	},
	FormatHTTPie: {
		"--auth": true, "-a": true, "--auth-type": true, "--bearer": true, "--timeout": true,
		"-o": true, "--output": true, "--session": true, "--pretty": true, "--style": true,
		"-p": true, "--print": true, "--verify": true, "--cert": true, "--cert-key": true,
		"--proxy": true, "--raw": true,
	},
	FormatPowerShell: {
		"-Method": true, "-Uri": true, "-Headers": true, "-Body": true, "-ContentType": true,
		"-TimeoutSec": true, "-Credential": true, "-UserAgent": true, "-Proxy": true,
		"-OutFile": true, "-MaximumRedirection": true, "-SessionVariable": true,
	},
	FormatFetch: {},
}

var (
	// jsSpaceInner is the body of the TS's `\s` class, spelled with code points so the character set
	// stays visible; jsSpaceClass wraps it for a positive class.
	jsSpaceInner = reClass(jsSpaceRunes)
	jsSpaceClass = "[" + jsSpaceInner + "]"
	// anyChar is `[\s\S]`: everything, newlines included.
	anyChar = "(?s:.)"

	schemeRe    = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9+.-]*://")
	localhostRe = regexp.MustCompile("^localhost(:[0-9]+)?(/|$)")
	portRe      = regexp.MustCompile("^:[0-9]+")
	// A bare host is a dot with something on each side and no `/:@` before the dot, so `jq .` and
	// `extra.txt` are not mistaken for one.
	hostRe = regexp.MustCompile("^[^" + jsSpaceInner + "/:@]+\\.[^" + jsSpaceInner + "/:@]")

	// headRe is the TS's /^([^\s;]+)\s*([\s\S]*)$/: the command word, then everything after it.
	headRe = regexp.MustCompile("^([^" + jsSpaceInner + ";]+)" + jsSpaceClass + "*(" + anyChar + "*)$")
	// fetchCallRe is /\bfetch\s*\(/: the call only counts at a word boundary, so `prefetch(` is not
	// one. The match starts at the `f`, which is where the argument list is measured from.
	fetchCallRe = regexp.MustCompile("\\bfetch" + jsSpaceClass + "*\\(")
	// fetchPrefixRe is the TS's FETCH_PREFIX: devtools hands over `const res = await fetch(…)`, and
	// a bare `fetch(` in prose is not a command.
	fetchPrefixRe = regexp.MustCompile("^(?:(?:await|return)" + jsSpaceClass + "+|" +
		"(?:const|let|var)" + jsSpaceClass + "+[A-Za-z0-9_$]+" + jsSpaceClass + "*=" + jsSpaceClass +
		"*(?:await" + jsSpaceClass + "+)?)*fetch" + jsSpaceClass + "*\\(")

	// promptRe is /^(?:\$\s+|PS\s[^>\n]*>\s*|>\s+|❯\s+)+/ — the prompts a copied line may still
	// carry. `PS>` alone does not match, and a line that keeps its prompt is not a command.
	promptRe = regexp.MustCompile("^(?:\\$" + jsSpaceClass + "+|PS" + jsSpaceClass + "[^>\n]*>" +
		jsSpaceClass + "*|>" + jsSpaceClass + "+|" + cursorChar + jsSpaceClass + "+)+")

	crlfRe       = regexp.MustCompile("\\r\\n?")
	sudoRe       = regexp.MustCompile("^sudo" + jsSpaceClass + "+")
	trailingFDRe = regexp.MustCompile(jsSpaceClass + "+[0-9]+" + jsSpaceClass + "*$")
)

// cursorChar is the `❯` of a fancy shell prompt.
const cursorChar = string(rune(0x276f))

// bom is U+FEFF, which a paste from some editors still carries at the front. It is written from its
// code point: a literal one would not survive as Go source, and it is also whitespace to JS.
const bom = string(rune(0xfeff))

// looksLikeURL is the generous test used to pick the URL out of the positionals.
func looksLikeURL(s string) bool {
	if schemeRe.MatchString(s) {
		return true
	}
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "{{") {
		return true
	}
	if localhostRe.MatchString(s) {
		return true
	}
	if portRe.MatchString(s) {
		return true
	}
	return hostRe.MatchString(s)
}

// isURLOperand is the strict test used where a wrong guess would change the meaning: a bare host is
// not enough.
func isURLOperand(s string) bool {
	if schemeRe.MatchString(s) {
		return true
	}
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "{{") {
		return true
	}
	return portRe.MatchString(s) || localhostRe.MatchString(s)
}

// detection is what detect worked out: which tool wrote the command, what is left of it once the
// command word is gone, and how that tool's shell quotes.
type detection struct {
	format  Format
	rest    string
	dialect dialect
}

// detect names the tool, or reports that this is not a command at all. A leading command word is
// required: the parse is meant for a pasted command, not for a URL typed into a body field.
func detect(text string) (detection, bool) {
	trimmed := jsTrim(text)

	if fetchCallRe.MatchString(trimmed) && fetchPrefixRe.MatchString(trimmed) {
		return detection{format: FormatFetch, rest: trimmed, dialect: posixDialect}, true
	}

	head := headRe.FindStringSubmatch(trimmed)
	if head == nil {
		return detection{}, false
	}
	name := strings.ToLower(head[1])
	rest := head[2]

	switch {
	case name == "curl" || name == "curl.exe":
		return detection{format: FormatCurl, rest: rest, dialect: shellDialect}, true
	case name == "wget" || name == "wget.exe":
		return detection{format: FormatWget, rest: rest, dialect: shellDialect}, true
	case strings.HasPrefix(name, "invoke-") || name == "irm" || name == "iwr":
		return detection{format: FormatPowerShell, rest: rest, dialect: powershellDialect}, true
	case name == "http" || name == "https" || name == "http.exe":
		// `http` is also an ordinary word, so it only counts when what follows looks like a request
		// line rather than prose.
		tokens, ok := tokenize(rest, posixDialect)
		if !ok {
			return detection{}, false
		}
		operand := firstOperand(tokens, valueFlags[FormatHTTPie])
		if !methods[strings.ToUpper(operand)] && !isURLOperand(operand) {
			return detection{}, false
		}
		return detection{format: FormatHTTPie, rest: rest, dialect: posixDialect}, true
	}
	return detection{}, false
}

// firstOperand is the first token that is neither a flag nor a flag's value.
func firstOperand(tokens []string, flags map[string]bool) string {
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if !strings.HasPrefix(token, "-") {
			return token
		}
		flag, inline := splitFlag(token)
		if inline == nil && flags[flag] {
			i++
		}
	}
	return ""
}

// normalizeInput strips what a copied command picks up from the terminal it came from: the BOM, the
// line endings, the prompt, sudo, and the redirections that are not part of the command.
func normalizeInput(text string) string {
	text = strings.TrimPrefix(text, bom)
	text = crlfRe.ReplaceAllString(text, "\n")
	text = promptRe.ReplaceAllString(text, "")
	text = sudoRe.ReplaceAllString(text, "")
	return stripRedirection(text)
}

// stripRedirection cuts the command at the first `>` or `|`, which starts a redirection or a pipe
// rather than belonging to the command. The cut is after the file descriptor number that `2>&1`
// and `2> out` leave behind, unless a quoted string is in the way.
func stripRedirection(text string) string {
	i := 0
	for i < len(text) {
		c := text[i]
		if c == '\'' || c == '"' {
			if r, ok := readQuoted(text, i, posixDialect); ok {
				i = r.next
			} else {
				i++
			}
			continue
		}
		if c == '>' || c == '|' {
			return trailingFDRe.ReplaceAllString(text[:i], "")
		}
		i++
	}
	return text
}
