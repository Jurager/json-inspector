package draft

import (
	"context"

	"json-inspector/internal/domain"
)

// Store keeps drafts between runs: a window that is closed mid-request opens on what it was
// composing. It is keyed by draft id, because a collection node will get a draft of its own — and
// by workspace, because the command line's id is the same word in every one of them.
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

// FileSource reads the bytes a request carries. A file body keeps a path and not the bytes — the
// way Postman keeps it — and the read happens at the moment of sending: a draft holding megabytes
// would be a megabyte in every snapshot, and a copy of a file that has changed since it was picked.
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

// AuthMaterializer turns what a level answered about its authorization into what actually goes on
// the wire. Which schemes exist and what they carry is the domain's business; how each of them
// works — a base64 credential, a signature over the request, a token fetched from an identity
// provider — is the outside world's, and that is what this port is for.
//
// The two calls differ in one thing only: Materialize may talk to the server, because getting a
// token for OAuth 2.0 and answering a Digest challenge are conversations. Project may not — it is
// what the window draws while a person is typing, and it answers with nothing where the
// conversation has not happened yet.
type AuthMaterializer interface {
	Materialize(ctx context.Context, auth domain.Auth, req domain.AuthRequest) (domain.AuthOutput,
		error)
	// Project is the same answer without the network: what the window draws as a derived row.
	Project(auth domain.Auth, req domain.AuthRequest) (domain.AuthOutput, error)
	// Absorb is the other direction: an edit to one of those rows, turned back into the scheme's
	// fields. A scheme whose rows the user may not edit is not asked, and one that is asked and
	// cannot answer says so rather than guessing at what the edit meant.
	Absorb(auth domain.Auth, target domain.RowKind, name, value string) (domain.Auth, error)

	// Obtain goes and gets whatever a scheme carries from somewhere else — a token from an identity
	// provider — and Forget drops it. Both are only asked of a scheme that fetches: one that carries
	// what it was given has nothing to obtain and nothing to drop.
	Obtain(ctx context.Context, auth domain.Auth) error
	Forget(auth domain.Auth)
	// Held is whether a scheme has a token at this moment, which is what the window draws «No
	// token» from. The token itself does not come back: see domain.AuthToken.
	Held(auth domain.Auth) domain.AuthToken
}
