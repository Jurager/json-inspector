package draft

// curl, written the way devtools writes it: headers as `-H`, the body as `--data-raw`, and no `-X`
// for a GET, which is what curl does anyway.

import (
	"strings"

	"json-inspector/internal/domain"
)

func renderCurl(url, method string, headers []domain.HeaderPair, body string) string {
	parts := []string{"curl"}
	if method != "GET" {
		parts = append(parts, "-X", method)
	}
	for _, h := range headers {
		parts = append(parts, "-H", shellQuote(h.Name+": "+h.Value))
	}
	if body != "" {
		parts = append(parts, "--data-raw", shellQuote(body))
	}
	parts = append(parts, shellQuote(url))
	return strings.Join(parts, " ")
}
