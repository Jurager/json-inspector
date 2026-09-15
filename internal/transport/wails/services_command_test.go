package wails

import (
	"context"
	"strings"
	"testing"

	"json-inspector/internal/command"
	"json-inspector/internal/domain"
)

// fakeRecords is the history as the export sees it: one request, with its body kept apart the way
// the store keeps it.
type fakeRecords struct {
	record domain.Record
	body   string
}

func (f fakeRecords) Record(context.Context, string) (domain.Record, error) {
	return f.record, nil
}

func (f fakeRecords) Body(context.Context, string, domain.BodySide) (string, error) {
	return f.body, nil
}

// fakeVars fills texts in the way the environments do: `{{host}}` becomes a value, and a secret
// becomes the mask. Which is which is the environments' business, and this is only the answer.
type fakeVars struct{}

func (fakeVars) SubstituteTexts(_ context.Context, texts []string, mask bool) ([]string, error) {
	out := make([]string, len(texts))
	for i, text := range texts {
		text = strings.ReplaceAll(text, "{{host}}", "api.example.com")
		if mask {
			text = strings.ReplaceAll(text, "{{token}}", "••••")
		}
		out[i] = text
	}
	return out, nil
}

func newCommandService() *CommandService {
	return NewCommandService(
		fakeRecords{
			record: domain.Record{
				RecordSummary: domain.RecordSummary{
					Method: "POST",
					URL:    "https://{{host}}/articles?token={{token}}",
				},
				RequestHeaders: []domain.HeaderPair{
					{Name: "Authorization", Value: "Bearer {{token}}"},
					{Name: "Accept", Value: "application/vnd.api+json"},
				},
			},
			body: `{"host":"{{host}}"}`,
		},
		fakeVars{},
	)
}

// An export outlives the moment of sending, so a secret leaves as its mask and everything else is
// resolved. The values themselves never cross back into the window: this is the whole reason the
// substitution happens here and not there.
func TestExportMasksASecretAndResolvesTheRest(t *testing.T) {
	text, err := newCommandService().Export(context.Background(), "rec-1", command.FormatCurl, false)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if strings.Contains(text, "s3cret") || strings.Contains(text, "{{token}}") {
		t.Errorf("export = %q, want no token and no secret in it", text)
	}
	if !strings.Contains(text, "Bearer ••••") {
		t.Errorf("export = %q, want the secret as its mask", text)
	}
	if !strings.Contains(text, "api.example.com") {
		t.Errorf("export = %q, want the other variables resolved", text)
	}
	if !strings.Contains(text, "-X POST") {
		t.Errorf("export = %q, want the method the record was sent with", text)
	}
}

// A snippet shared with the tokens intact is the other half of the same gesture: the variables are
// resolved where the request is sent, not where it is copied.
func TestExportCanKeepTheTokens(t *testing.T) {
	text, err := newCommandService().Export(context.Background(), "rec-1", command.FormatCurl, true)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if !strings.Contains(text, "{{token}}") || !strings.Contains(text, "{{host}}") {
		t.Errorf("export = %q, want the tokens as they were written", text)
	}
	if strings.Contains(text, "••••") {
		t.Errorf("export = %q, want no mask where the tokens are kept", text)
	}
}

// Pasting something that is not a command is not a failure: the window lets the text through.
func TestParseOfSomethingElseIsNotAnError(t *testing.T) {
	svc := newCommandService()

	for _, text := range []string{"https://api.example.com/articles", "просто текст", ""} {
		result, err := svc.Parse(context.Background(), text)
		if err != nil {
			t.Fatalf("Parse(%q): %v", text, err)
		}
		if result.Kind != command.KindNone {
			t.Errorf("Parse(%q) = %q, want none", text, result.Kind)
		}
	}

	result, err := svc.Parse(context.Background(),
		"curl -X POST https://api.example.com -d '{\"a\":1}'")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if result.Kind != command.KindOK || result.Format != command.FormatCurl ||
		result.Request.Method != "POST" {
		t.Errorf("Parse = %+v, want a POST read as curl", result)
	}
}
