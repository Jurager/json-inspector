package draft

import (
	"context"

	"json-inspector/internal/domain"
)

// Store keeps drafts between runs: a window that is closed mid-request opens on what it was
// composing. It is keyed by draft id, because a collection node will get a draft of its own — and by
// workspace, because the command line's id is the same word in every one of them.
type Store interface {
	Draft(ctx context.Context, workspaceID string, id domain.DraftID) (domain.Draft, error)
	SaveDraft(ctx context.Context, workspaceID string, draft domain.Draft) error
}

// Scope answers which workspace the window is showing. It is a port of this feature's own rather
// than a call into the workspace use case: features never import each other, and this one only
// needs the name of the space it is working in.
type Scope interface {
	ActiveWorkspace(ctx context.Context) (string, error)
}

// FileSource reads the bytes a request carries. A file body keeps a path and not the bytes — the way
// Postman keeps it — and the read happens at the moment of sending: a draft holding megabytes would
// be a megabyte in every snapshot, and a copy of a file that has changed since it was picked.
//
// The size limit belongs to the port and not to the caller: a caller that forgets to check is
// exactly the failure this exists to prevent.
type FileSource interface {
	Read(path string) ([]byte, error)
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
