package account

import (
	"context"

	"json-inspector/internal/domain"
)

// Store is the one row this feature keeps: who the app is signed in as. It is declared here, next
// to its user, so the feature knows nothing about SQLite.
type Store interface {
	// Account is the account as it was last written. Nothing is signed in and nothing ever was
	// answers ErrNotFound, which is an ordinary state — the app is usable without an account.
	Account(ctx context.Context) (domain.Account, error)
	// Save writes the account and the credential that keeps it. The refresh token travels beside the
	// account rather than inside it, the way a secret travels beside the row that means it. An empty
	// token means "leave whatever is there": changing the address is not signing out.
	Save(ctx context.Context, account domain.Account, refreshToken string) error
	// RefreshToken is the credential on its own, because that is how it is used: the account it
	// belongs to is what a window draws, and this is what a call to the server carries.
	RefreshToken(ctx context.Context) (string, error)
	// Forget drops the lot: the account, the token and the session id. What stays is the address of
	// the server, which is a choice rather than a credential.
	Forget(ctx context.Context) error
}

// Server is the other side: the OIDC endpoints a device signs in through and the API of the account
// it signs in to. One interface for one server, because a caller can never have half of it.
//
// Every method takes the address it is about: the account is what the app keeps, and the client
// underneath is stateless — the same process may well talk to a self-hosted server today and to
// another one tomorrow.
type Server interface {
	// Authorize asks for a device code: what the person has to confirm, and where.
	Authorize(ctx context.Context, address string, device domain.Device) (domain.Challenge, error)
	// Tokens waits for the person to confirm the code and answers the pair of tokens. It returns when
	// the code is confirmed, refused, expired, or the context is cancelled — there is no third thing
	// for it to do in between.
	Tokens(ctx context.Context, address string, challenge domain.Challenge) (domain.Tokens, error)
	// Refresh trades a refresh token for a new pair, which is how a device stays signed in without
	// anybody typing anything.
	Refresh(ctx context.Context, address, refresh string) (domain.Tokens, error)

	Me(ctx context.Context, address, access string) (domain.Account, error)
	Sessions(ctx context.Context, address, access string) ([]domain.Session, error)
	EndSession(ctx context.Context, address, access, sessionID string) ([]domain.Session, error)
	EndOthers(ctx context.Context, address, access string) ([]domain.Session, error)
	SignOut(ctx context.Context, address, access string) error
	DeleteAccount(ctx context.Context, address, access string) error
}

// Browser is where a person confirms the code. The app never asks for a password, so this is the
// whole of the sign-in as far as the window is concerned: it hands a URL to something that can show
// a page.
type Browser interface {
	// OpenURL is the same name the window's own port uses, so the window is what serves this one
	// without an adapter in between.
	OpenURL(url string) error
}

// Notifier publishes what happened to whoever is listening. Each window draws the same account, so
// this is a broadcast and not an answer to whoever asked.
type Notifier interface {
	Publish(topic string, payload any)
}
