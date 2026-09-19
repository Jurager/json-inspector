package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"json-inspector/internal/domain"
)

// The account's row — the only row in the schema that is about the other side of the wire. It is
// written by the sign-in and read by every window that draws who this installation belongs to.

// Account reads the account. Nothing signed in answers ErrNotFound, which is the ordinary state of
// a local app rather than a problem with the database.
func (s *Store) Account(ctx context.Context) (domain.Account, error) {
	const query = `
		SELECT server, user_id, email, name, plan, seats, session_id, signed_in_at
		  FROM account WHERE id = 1`

	var (
		account    domain.Account
		signedInAt int64
	)
	err := s.db.QueryRowContext(ctx, query).Scan(&account.Server, &account.UserID, &account.Email,
		&account.Name, &account.Plan, &account.Seats, &account.SessionID, &signedInAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Account{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Account{}, fmt.Errorf("reading the account: %w", err)
	}
	if signedInAt != 0 {
		account.SignedInAt = time.UnixMilli(signedInAt)
	}
	return account, nil
}

// Save writes the account, keeping the refresh token when none is given: changing the address of
// the server is not a reason to throw away the credential that signs this device in.
func (s *Store) Save(ctx context.Context, account domain.Account, refreshToken string) error {
	const query = `
		INSERT INTO account (id, server, user_id, email, name, plan, seats, session_id,
		                     refresh_token, signed_in_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			server = excluded.server, user_id = excluded.user_id, email = excluded.email,
			name = excluded.name, plan = excluded.plan, seats = excluded.seats,
			session_id = excluded.session_id, signed_in_at = excluded.signed_in_at,
			refresh_token = CASE WHEN excluded.refresh_token = ''
				THEN account.refresh_token ELSE excluded.refresh_token END`

	_, err := s.db.ExecContext(ctx, query, account.Server, account.UserID, account.Email, account.Name,
		account.Plan, account.Seats, account.SessionID, refreshToken, account.SignedInAt.UnixMilli())
	if err != nil {
		return fmt.Errorf("saving the account: %w", err)
	}
	return nil
}

// RefreshToken is the credential on its own: what a call to the server carries, and what buys the
// next access token.
func (s *Store) RefreshToken(ctx context.Context) (string, error) {
	const query = `SELECT refresh_token FROM account WHERE id = 1`

	var token string
	err := s.db.QueryRowContext(ctx, query).Scan(&token)
	if errors.Is(err, sql.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	// The error is read before the emptiness of the token: a scan that failed leaves the token empty,
	// and calling a broken database "no account" would sign the user out over a disk failure.
	if err != nil {
		return "", fmt.Errorf("reading the refresh token: %w", err)
	}
	if token == "" {
		return "", domain.ErrNotFound
	}
	return token, nil
}

// Forget drops the whole row: the account, the token and the session it named.
func (s *Store) Forget(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM account WHERE id = 1`); err != nil {
		return fmt.Errorf("forgetting the account: %w", err)
	}
	return nil
}
