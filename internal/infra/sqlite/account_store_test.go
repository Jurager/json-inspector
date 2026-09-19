package sqlite

import (
	"context"
	"errors"
	"testing"

	"json-inspector/internal/domain"
)

// A database that cannot be read is not an account that is not there, and the two have to answer
// differently: "no account" is the ordinary state of a local app, while a failure has to reach the
// caller that would otherwise decide the user is not signed in and sign them out for good.
func TestRefreshTokenTellsAFailureFromNoAccount(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	if _, err := store.RefreshToken(ctx); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("with nothing signed in = %v, want ErrNotFound", err)
	}

	// The database goes away under the call, which is the shortest way to the failure the ordering
	// of the checks used to swallow.
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := store.RefreshToken(ctx); err == nil || errors.Is(err, domain.ErrNotFound) {
		t.Errorf("on a database that is gone = %v, want a failure and not ErrNotFound", err)
	}
}
