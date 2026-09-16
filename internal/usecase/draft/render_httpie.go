package draft

// httpie, whose grammar puts the body behind `--raw=` and the headers after the URL as
// `Name:value` items — the same items the reader takes apart.

import (
	"strings"

	"json-inspector/internal/domain"
)

func renderHTTPie(url, method string, headers []domain.HeaderPair, body string) string {
	parts := []string{"http"}
	if body != "" {
		parts = append(parts, "--raw="+shellQuote(body))
	}
	parts = append(parts, method, shellQuote(url))
	for _, h := range headers {
		parts = append(parts, shellQuote(h.Name+":"+h.Value))
	}
	return strings.Join(parts, " ")
}
