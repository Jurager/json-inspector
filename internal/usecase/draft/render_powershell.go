package draft

// Invoke-RestMethod, whose headers are one `@{ … }` hashtable and whose quotes double rather than
// close and reopen.

import (
	"strings"

	"json-inspector/internal/domain"
)

func renderPowerShell(url, method string, headers []domain.HeaderPair, body string) string {
	parts := []string{"Invoke-RestMethod -Method " + method + " -Uri " + psQuote(url)}
	if len(headers) > 0 {
		pairs := make([]string, 0, len(headers))
		for _, h := range headers {
			pairs = append(pairs, psQuote(h.Name)+" = "+psQuote(h.Value))
		}
		parts = append(parts, "-Headers @{ "+strings.Join(pairs, "; ")+" }")
	}
	if body != "" {
		parts = append(parts, "-Body "+psQuote(body))
	}
	return strings.Join(parts, " ")
}
