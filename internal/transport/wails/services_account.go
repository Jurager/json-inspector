package wails

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/usecase/account"
)

// AccountService is the account as the window sees it: who the app is signed in as, the sign-in
// itself, and the devices it is signed in on.
//
// Every method answers the whole state rather than the piece that changed — a window drawing the
// account wants one answer, not a puzzle of them — and every window is told as well, because the
// settings window is not the only one that draws it.
type AccountService struct {
	account *account.UseCase
}

func NewAccountService(uc *account.UseCase) *AccountService {
	return &AccountService{account: uc}
}

// State is what the settings window draws: nothing signed in, a sign-in in progress, or an account.
func (s *AccountService) State(ctx context.Context) (account.State, error) {
	return s.account.State(ctx)
}

// SetServer remembers a server other than the one the app would use by default. It is kept even
// when nobody is signed in, and it does not sign anybody out.
func (s *AccountService) SetServer(ctx context.Context, server string) (account.State, error) {
	return s.account.SetServer(ctx, server)
}

// Begin starts a sign-in: the server hands over a code, the browser opens on the page where it is
// confirmed, and the answer comes back as an event — see TopicChanged. The call returns as soon as
// there is a code to show, because waiting is the person's business and not the window's.
func (s *AccountService) Begin(ctx context.Context, server string) (account.State, error) {
	return s.account.Begin(ctx, server)
}

// Cancel stops waiting for a code that nobody is going to confirm. The code itself lives on until
// it runs out on its own: the app cannot call it back.
func (s *AccountService) Cancel(ctx context.Context) (account.State, error) {
	return s.account.Cancel(ctx)
}

// SignOut forgets the account on this machine. It works with the server out of reach: this is a
// local app, and being unable to reach anybody is not a reason to stay signed in.
func (s *AccountService) SignOut(ctx context.Context) (account.State, error) {
	return s.account.SignOut(ctx)
}

// Sessions is where else the account is signed in. It is a call to the server, so it can fail with
// the server being away — which the window says in its own words rather than showing an empty list.
func (s *AccountService) Sessions(ctx context.Context) ([]domain.Session, error) {
	return s.account.Sessions(ctx)
}

func (s *AccountService) EndSession(ctx context.Context,
	sessionID string) ([]domain.Session, error) {
	return s.account.EndSession(ctx, sessionID)
}

func (s *AccountService) EndOthers(ctx context.Context) ([]domain.Session, error) {
	return s.account.EndOthers(ctx)
}

// DeleteAccount leaves the server for good. The collections on this machine stay: they were always
// local, and leaving an account is not the same as erasing a drive.
func (s *AccountService) DeleteAccount(ctx context.Context) (account.State, error) {
	return s.account.DeleteAccount(ctx)
}
