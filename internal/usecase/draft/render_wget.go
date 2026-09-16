package draft

// wget, whose flags carry their value with an `=`: `--method=POST`, `--header=…`, `--body-data=…`.

import (
	"strings"

	"json-inspector/internal/domain"
)

func renderWget(url, method string, headers []domain.HeaderPair, body string) string {
	parts := []string{"wget"}
	if method != "GET" {
		parts = append(parts, "--method="+method)
	}
	for _, h := range headers {
		parts = append(parts, "--header="+shellQuote(h.Name+": "+h.Value))
	}
	if body != "" {
		parts = append(parts, "--body-data="+shellQuote(body))
	}
	parts = append(parts, shellQuote(url))
	return strings.Join(parts, " ")
}
