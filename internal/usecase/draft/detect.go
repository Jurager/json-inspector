package draft

import (
	"regexp"
	"strings"
)

// curlFlags are the flags that take a value. A flag listed here is a flag, so its value is not an
// operand: without the table `curl -o out.json URL` would offer `out.json` as the URL.
var curlFlags = map[string]bool{
	"-X": true, "--request": true, "-H": true, "--header": true, "-d": true, "--data": true,
	"--data-raw": true, "--data-binary": true, "--data-ascii": true, "--data-urlencode": true,
	"--url": true, "-u": true, "--user": true, "-A": true, "--user-agent": true, "-b": true,
	"--cookie": true, "-c": true, "--cookie-jar": true, "-e": true, "--referer": true,
	"-x": true, "--proxy": true, "-o": true, "--output": true, "-T": true, "--upload-file": true,
	"-F": true, "--form": true, "--form-string": true, "-m": true, "--max-time": true,
	"--connect-timeout": true, "--retry": true, "--resolve": true, "--cert": true, "--key": true,
	"--cacert": true, "--capath": true, "--interface": true, "--limit-rate": true, "-w": true,
	"--write-out": true, "-K": true, "--config": true, "--json": true, "--oauth2-bearer": true,
}

var (
	// anyChar is `[\s\S]`: everything, newlines included.
	anyChar = "(?s:.)"

	schemeRe    = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9+.-]*://")
	localhostRe = regexp.MustCompile("^localhost(:[0-9]+)?(/|$)")
	portRe      = regexp.MustCompile("^:[0-9]+")
	// A bare host is a dot with something on each side and no `/:@` before the dot, so `jq .` and
	// `extra.txt` are not mistaken for one.
	hostRe = regexp.MustCompile("^[^" + spaceInner + "/:@]+\\.[^" + spaceInner + "/:@]")

	// headRe is /^([^\s;]+)\s*([\s\S]*)$/: the command word, then everything after it.
	headRe = regexp.MustCompile("^([^" + spaceInner + ";]+)" + spaceClass + "*(" + anyChar + "*)$")

	// promptRe is /^(?:\$\s+|PS\s[^>\n]*>\s*|>\s+|❯\s+)+/ — the prompts a copied line may still
	// carry. `PS>` alone does not match, and a line that keeps its prompt is not a command.
	promptRe = regexp.MustCompile("^(?:\\$" + spaceClass + "+|PS" + spaceClass + "[^>\n]*>" +
		spaceClass + "*|>" + spaceClass + "+|" + cursorChar + spaceClass + "+)+")

	crlfRe       = regexp.MustCompile(`\r\n?`)
	sudoRe       = regexp.MustCompile("^sudo" + spaceClass + "+")
	trailingFDRe = regexp.MustCompile(spaceClass + "+[0-9]+" + spaceClass + "*$")
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

// detect recognises a curl command and answers what is left of it once the command word is gone. A
// leading command word is required: this is meant for a pasted command, not for a URL typed into a
// body field, and a paste that is not a command has to come back as one so the window can leave the
// field alone.
//
// `curl` is the only tool read, because it is the only one a person copies a request out of: the
// other notations this app can *write* — fetch, wget, httpie, PowerShell — are ways of carrying a
// request away, and nobody pastes a PowerShell line into an address field to get a request back.
func detect(text string) (string, bool) {
	head := headRe.FindStringSubmatch(trimSpace(text))
	if head == nil {
		return "", false
	}
	switch strings.ToLower(head[1]) {
	case "curl", "curl.exe":
		return head[2], true
	}
	return "", false
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
			if r, ok := readQuoted(text, i); ok {
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
