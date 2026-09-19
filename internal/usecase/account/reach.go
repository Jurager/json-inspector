package account

import (
	"context"
	"sync"
	"time"

	"json-inspector/internal/domain"
)

// Reachability is what the last call to the server came to: whether the other side answered, and
// when it last did — a failed attempt does not move that moment, because «последняя связь» is not
// «последняя попытка». It is not part of the account: the account is a row this app keeps, and a
// server that is down or a laptop that is offline is weather rather than a fact about the person.
//
// Checked tells "not asked yet" from "did not answer", because a window words them differently:
// before the first call there is nothing to say, and after it there is.
type Reachability struct {
	Checked   bool      `json:"checked"`
	Reachable bool      `json:"reachable"`
	At        time.Time `json:"at"`
}

// reach is where the outcome of the last call is kept.
type reach struct {
	mu    sync.Mutex
	state Reachability
}

// note writes down how a call ended. A refusal counts as reached: a server that says no is a server
// that is there, and a person's next move is different in the two cases.
func (r *reach) note(at time.Time, err error) {
	reached := domain.CodeOf(err) != domain.CodeServerUnreachable

	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.Checked, r.state.Reachable = true, reached
	// The moment kept is when the server last *answered* and not when it was last asked: a window
	// draws it as «последняя связь», and an attempt that never got there is not one.
	if reached {
		r.state.At = at
	}
}

func (r *reach) now() Reachability {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

// watched is the server port with a notebook: every call through it writes down whether the other
// side answered.
//
// Every method is written out here for one reason. Recording the outcome at each call site instead
// would mean nine lines that a tenth call could be added without — and the cost of forgetting is
// not a missing log line but a window that goes on drawing an account nobody can reach.
type watched struct {
	Server
	uc *UseCase
}

func (w watched) Authorize(
	ctx context.Context,
	address string,
	device domain.Device,
) (domain.Challenge, error) {
	challenge, err := w.Server.Authorize(ctx, address, device)
	w.uc.noteCall(err)
	return challenge, err
}

func (w watched) Tokens(
	ctx context.Context,
	address string,
	challenge domain.Challenge,
) (domain.Tokens, error) {
	tokens, err := w.Server.Tokens(ctx, address, challenge)
	w.uc.noteCall(err)
	return tokens, err
}

func (w watched) Refresh(ctx context.Context, address, refresh string) (domain.Tokens, error) {
	tokens, err := w.Server.Refresh(ctx, address, refresh)
	w.uc.noteCall(err)
	return tokens, err
}

func (w watched) Me(ctx context.Context, address, access string) (domain.Account, error) {
	account, err := w.Server.Me(ctx, address, access)
	w.uc.noteCall(err)
	return account, err
}

func (w watched) Sessions(ctx context.Context, address, access string) ([]domain.Session, error) {
	sessions, err := w.Server.Sessions(ctx, address, access)
	w.uc.noteCall(err)
	return sessions, err
}

func (w watched) EndSession(
	ctx context.Context,
	address, access, sessionID string,
) ([]domain.Session, error) {
	left, err := w.Server.EndSession(ctx, address, access, sessionID)
	w.uc.noteCall(err)
	return left, err
}

func (w watched) EndOthers(ctx context.Context, address, access string) ([]domain.Session, error) {
	left, err := w.Server.EndOthers(ctx, address, access)
	w.uc.noteCall(err)
	return left, err
}

func (w watched) SignOut(ctx context.Context, address, access string) error {
	err := w.Server.SignOut(ctx, address, access)
	w.uc.noteCall(err)
	return err
}

func (w watched) DeleteAccount(ctx context.Context, address, access string) error {
	err := w.Server.DeleteAccount(ctx, address, access)
	w.uc.noteCall(err)
	return err
}
