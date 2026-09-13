package draft

import (
	"context"

	"json-inspector/internal/domain"
)

// Store keeps drafts between runs: a window that is closed mid-request opens on what it was
// composing. It is keyed by draft id, because a collection node will get a draft of its own.
type Store interface {
	Draft(ctx context.Context, id domain.DraftID) (domain.Draft, error)
	SaveDraft(ctx context.Context, draft domain.Draft) error
}

// VariableSource is what the draft asks about `{{tokens}}`: which of them mean nothing, and what a
// list of texts looks like with them filled in. Both answers stay with the feature that owns the
// values — the draft itself only ever holds the text the user typed.
//
// The calls take a list because a request is a list: one round of resolving covers its URL, its
// body, every header and its cookies, and the answers stay consistent with each other.
type VariableSource interface {
	Missing(ctx context.Context, texts []string) ([]string, error)
	SubstituteTexts(ctx context.Context, texts []string, mask bool) ([]string, error)
}
