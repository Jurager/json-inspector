package wails

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/usecase/draft"
	"json-inspector/internal/usecase/record"
)

// requestMask is a request seen the way everything that outlives the send sees it: with a secret's
// value left as its mask. A pre-request script may have changed the request, and masking that text
// happens in the feature that owns the values — so the composition asks it again, once, for the
// request that actually goes out.
type requestMask struct {
	drafts *draft.UseCase
}

var _ record.Masker = requestMask{}

func (m requestMask) Mask(ctx context.Context, req domain.ScriptRequest) (record.Masked, error) {
	prepared, err := m.drafts.Prepare(ctx, draft.Seed{
		Method:  req.Method,
		URL:     req.URL,
		Body:    req.Body,
		Headers: req.Headers,
	})
	if err != nil {
		return record.Masked{}, err
	}
	return record.Masked{
		URL:     prepared.MaskedURL,
		Headers: prepared.MaskedHeaders,
		Body:    prepared.MaskedBody,
	}, nil
}
