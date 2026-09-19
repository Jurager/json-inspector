// Package account owns the account: who the app is signed in as, where else it is signed in, and
// the way in — a code a person confirms in their browser.
//
// The app is a local tool first and stays one without an account. Nothing here is on the path of
// opening a collection or sending a request; what it does is keep the one row that says who this
// installation belongs to, and answer the settings window about it.
//
// The tokens themselves are not this feature's business either — a library speaks OIDC — but the
// decision about *when* a token is needed, and what a refusal means, is the shape of what a person
// experiences, and that is here.
package account

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

// TopicChanged is published whenever the account moves: signed in, signed out, a device ended.
// Every window draws the same account, so the one that made the change is not the one that tells
// the rest.
const TopicChanged = "account:changed"

// State is what a window needs to draw the account: whether anybody is signed in, and what is
// happening right now — a sign-in in progress is a modal with a spinner, not an empty pane.
type State struct {
	SignedIn bool           `json:"signedIn"`
	Account  domain.Account `json:"account"`
	// Waiting is a sign-in the app is in the middle of: the code is out, the browser is open, and
	// nothing has happened yet.
	Waiting   bool             `json:"waiting"`
	Challenge domain.Challenge `json:"challenge"`
	// Failure is why the last attempt came to nothing, in the app's own codes. The window words it.
	Failure *domain.Failure `json:"failure,omitempty"`
	// Reachability is what the last call to the server came to. A window draws the account from the
	// row and says this beside it: the account is what this app keeps, and a server it cannot reach
	// is not the same thing as nobody being signed in.
	Reachability Reachability `json:"reachability"`
}

type UseCase struct {
	store    Store
	server   Server
	browser  Browser
	notifier Notifier
	now      func() time.Time

	mu sync.Mutex
	// access is the short-lived token, and it lives in memory only: it is good for an hour, and a
	// token written to a file is one more secret on the disk for no gain.
	access    string
	expiresAt time.Time
	// waiting is the sign-in in progress, if there is one.
	waiting   bool
	challenge domain.Challenge
	cancel    context.CancelFunc
	// reach is whether the server answered the last call, for the window to draw beside the account.
	reach reach
}

func NewUseCase(store Store, server Server, browser Browser, notifier Notifier) *UseCase {
	u := &UseCase{store: store, browser: browser, notifier: notifier, now: time.Now}
	// The port is wrapped before anything can use it: the outcome of every call is written down by
	// the wrapper, and nothing in this file has to remember to do it — see watched.
	u.server = watched{Server: server, uc: u}
	return u
}

// noteCall is where a call's outcome lands. The wrapper calls it; nothing else should.
func (u *UseCase) noteCall(err error) {
	u.reach.note(u.now(), err)
}

// State answers what the window draws, reading the account from the store on every call: the row is
// the truth, and a copy kept here would be a second one.
func (u *UseCase) State(ctx context.Context) (State, error) {
	stored, err := u.store.Account(ctx)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return State{}, err
	}

	u.mu.Lock()
	waiting, challenge := u.waiting, u.challenge
	u.mu.Unlock()

	// What travels is the choice and not the address in use: the window draws a server of somebody's
	// own when there is one, and the product's own server is not a thing to draw.
	return State{
		SignedIn:     stored.SignedIn(),
		Account:      stored,
		Waiting:      waiting,
		Challenge:    challenge,
		Reachability: u.reach.now(),
	}, nil
}

// Check asks the server whether it is there, and takes the answer it gives about the account along
// with it: `Me` is the lightest request that also carries the email and the plan, so a server that
// has come back is drawn with what it says now rather than with what it said last time.
//
// A server that does not answer is an answer to this question rather than a failure of the call:
// the verdict travels in State.Reachability, which is what the windows draw. An error out of here
// is the store, or the disk — something this call was not asking about.
func (u *UseCase) Check(ctx context.Context) (State, error) {
	stored, access, err := u.signedIn(ctx)
	// A refusal names one of three things — nobody is signed in, the server did not answer, the
	// server said no — and each of them is what this call asked. Anything else is not ours to hush.
	if err != nil && domain.AsFailure(err) == nil {
		return State{}, err
	}

	if err == nil {
		if fetched, err := u.server.Me(ctx, serverAddress(stored.Server), access); err == nil {
			// The choice of server and the moment this device signed in are this app's own facts, not
			// the server's answers, so they stay as the row had them. The token is untouched too:
			// ending a sign-in is the only thing that changes it.
			fetched.Server, fetched.SignedInAt = stored.Server, stored.SignedInAt
			if err := u.store.Save(ctx, fetched, ""); err != nil {
				return State{}, err
			}
		}
	}
	return u.announce(ctx)
}

// SetServer remembers the address of the server to sign in to. It is kept even when nothing is
// signed in: a person chose it once, and asking again would be asking twice.
//
// An empty address is not a mistake but an answer — "the program's own server" — and it is kept as
// the empty string rather than written out, so that a build which later points somewhere else takes
// everybody who never chose with it.
func (u *UseCase) SetServer(ctx context.Context, address string) (State, error) {
	stored, err := u.store.Account(ctx)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return State{}, err
	}
	stored.Server = chosenAddress(address)

	// The refresh token is not touched: changing the address is not signing out.
	if err := u.store.Save(ctx, stored, ""); err != nil {
		return State{}, err
	}
	return u.announce(ctx)
}

// Begin starts a sign-in: it asks the server for a code, opens the browser on the page where the
// code is confirmed, and leaves a watch running that finishes the job when somebody does.
//
// It answers as soon as there is a code to show. Waiting for the person is what the watch is for,
// and a call that blocked until they got round to it would hold the window for as long as they
// took.
func (u *UseCase) Begin(ctx context.Context, address string) (State, error) {
	// Nobody naming a server means the program's own: a person who has never heard of self-hosting
	// presses the button and is signed in. The row keeps the *choice* — nothing, in that case — so
	// that a window draws a server of somebody's own only when there is one.
	to := serverAddress(address)
	u.stop()

	device := u.device()
	challenge, err := u.server.Authorize(ctx, to, device)
	if err != nil {
		return State{}, err
	}

	// The browser is opened by the app and not by the window: the page has to be a real one, outside
	// the app, because that is where the person's session on the server lives. A browser that will not
	// open is not a failure to report — the code and its address are on the screen, and they are
	// enough to get there by hand.
	_ = u.browser.OpenURL(challenge.URL)

	stored, err := u.store.Account(ctx)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return State{}, err
	}
	stored.Server = chosenAddress(address)
	if err := u.store.Save(ctx, stored, ""); err != nil {
		return State{}, err
	}

	watching, stop := context.WithCancel(context.WithoutCancel(ctx))
	u.mu.Lock()
	u.waiting, u.challenge, u.cancel = true, challenge, stop
	u.mu.Unlock()

	go u.watch(watching, to, challenge)

	state, err := u.State(ctx)
	if err != nil {
		return State{}, err
	}
	u.notifier.Publish(TopicChanged, state)
	return state, nil
}

// Cancel stops a sign-in that is in progress. The code stays valid on the server until it runs out
// on its own: the app cannot revoke what it asked for, and pretending otherwise would be a lie
// about what cancelling means.
func (u *UseCase) Cancel(ctx context.Context) (State, error) {
	u.stop()
	return u.announce(ctx)
}

// SignOut forgets the account. It tells the server first so that the device disappears from the
// list of the person's devices — but a server that cannot be reached does not keep somebody signed
// in here: this is a local app, and signing out locally has to work offline.
func (u *UseCase) SignOut(ctx context.Context) (State, error) {
	stored, err := u.store.Account(ctx)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return State{}, err
	}

	if stored.SignedIn() {
		if access, err := u.token(ctx, stored); err == nil {
			_ = u.server.SignOut(ctx, serverAddress(stored.Server), access)
		}
	}

	if err := u.forgetAccount(ctx, stored.Server); err != nil {
		return State{}, err
	}
	return u.announce(ctx)
}

// Sessions is where else the account is signed in — the question the settings window asks, and the
// one that has to work while the app is offline: the answer is "the server could not be reached",
// not a broken screen.
func (u *UseCase) Sessions(ctx context.Context) ([]domain.Session, error) {
	stored, access, err := u.signedIn(ctx)
	if err != nil {
		return nil, err
	}
	sessions, err := u.server.Sessions(ctx, serverAddress(stored.Server), access)
	if err != nil {
		return nil, err
	}
	return sessions, nil
}

// EndSession ends one device — the one a person no longer has, or no longer trusts. Ending this
// device is allowed and is what signing out is; the app then forgets the account like any other
// sign-out, because a session that is gone is not a session.
func (u *UseCase) EndSession(ctx context.Context, sessionID string) ([]domain.Session, error) {
	stored, access, err := u.signedIn(ctx)
	if err != nil {
		return nil, err
	}

	left, err := u.server.EndSession(ctx, serverAddress(stored.Server), access, sessionID)
	if err != nil {
		return nil, err
	}
	if sessionID == stored.SessionID {
		if err := u.forgetAccount(ctx, stored.Server); err != nil {
			return nil, err
		}
		if _, err := u.announce(ctx); err != nil {
			return nil, err
		}
	}
	return left, nil
}

// EndOthers ends every device but this one, which is what a person does when they think somebody
// else has their token. The one asking is kept by its session id.
func (u *UseCase) EndOthers(ctx context.Context) ([]domain.Session, error) {
	stored, access, err := u.signedIn(ctx)
	if err != nil {
		return nil, err
	}
	return u.server.EndOthers(ctx, serverAddress(stored.Server), access)
}

// DeleteAccount leaves the server for good: the account, its devices and everything about it. The
// app forgets the account afterwards, and the collections on this machine stay — they were always
// local, and leaving is not the same as erasing.
func (u *UseCase) DeleteAccount(ctx context.Context) (State, error) {
	stored, access, err := u.signedIn(ctx)
	if err != nil {
		return State{}, err
	}
	if err := u.server.DeleteAccount(ctx, serverAddress(stored.Server), access); err != nil {
		return State{}, err
	}

	if err := u.forgetAccount(ctx, stored.Server); err != nil {
		return State{}, err
	}
	return u.announce(ctx)
}

// watch waits for the person to confirm the code, and finishes the sign-in when they do.
//
// It is a goroutine because the window is not: the modal shows a code and a spinner, and everything
// that happens next happens on the far side of a browser the app does not control.
func (u *UseCase) watch(ctx context.Context, address string, challenge domain.Challenge) {
	tokens, err := u.server.Tokens(ctx, address, challenge)
	if err == nil {
		err = u.remember(ctx, address, tokens)
	}

	var failure *domain.Failure
	if err != nil {
		if ctx.Err() != nil {
			// Cancelled by the person, or by the app closing: not a refusal, and not news either —
			// whatever was showing the spinner has stopped on its own.
			u.finish(nil)
			return
		}
		failure = failureOf(err)
	}
	u.finish(failure)
}

// remember writes the account a finished sign-in produced. The account itself comes from the
// server: the email and the plan are its answers, and the session id it hands over is what makes
// this device recognisable in the list of them.
func (u *UseCase) remember(ctx context.Context, address string, tokens domain.Tokens) error {
	stored, err := u.server.Me(ctx, address, tokens.Access)
	if err != nil {
		return err
	}
	// The choice, not the address that was used: signing in to the program's own server is not
	// choosing a server, and a row that said otherwise would have the window draw an address for an
	// installation that never named one.
	stored.Server = ""
	if address != platform.DefaultServer {
		stored.Server = address
	}
	stored.SignedInAt = u.now()

	u.mu.Lock()
	u.access, u.expiresAt = tokens.Access, tokens.ExpiresAt
	u.mu.Unlock()

	return u.store.Save(ctx, stored, tokens.Refresh)
}

// finish closes a sign-in: the watch is over, and whoever is looking is told how it went.
func (u *UseCase) finish(failure *domain.Failure) {
	u.mu.Lock()
	u.waiting, u.challenge, u.cancel = false, domain.Challenge{}, nil
	u.mu.Unlock()

	state, err := u.State(context.Background())
	if err != nil {
		return
	}
	state.Failure = failure
	u.notifier.Publish(TopicChanged, state)
}

// signedIn is the account and a token that works, or a refusal naming which of the two is missing.
func (u *UseCase) signedIn(ctx context.Context) (domain.Account, string, error) {
	stored, err := u.store.Account(ctx)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return domain.Account{}, "", err
	}
	if !stored.SignedIn() {
		return domain.Account{}, "", domain.Refuse(domain.CodeNotSignedIn, domain.ErrNotAllowed, nil)
	}

	access, err := u.token(ctx, stored)
	if err != nil {
		return domain.Account{}, "", err
	}
	return stored, access, nil
}

// token is a token that works: the one in memory while it lasts, and a fresh one from the refresh
// token when it does not. A server that cannot be reached is told apart from one that said no — the
// first is the weather, and the second means the account is gone.
func (u *UseCase) token(ctx context.Context, stored domain.Account) (string, error) {
	u.mu.Lock()
	access, expiresAt := u.access, u.expiresAt
	u.mu.Unlock()

	// A minute of slack: a token that expires while the call is in flight is a call that comes back
	// unauthorised for no reason the person could understand.
	if access != "" && u.now().Add(time.Minute).Before(expiresAt) {
		return access, nil
	}

	refresh, err := u.store.RefreshToken(ctx)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return "", err
	}

	tokens, err := u.server.Refresh(ctx, serverAddress(stored.Server), refresh)
	if err != nil {
		if domain.CodeOf(err) == domain.CodeServerRefused {
			// The server says this refresh token is no good: the account was ended somewhere else, or
			// the token was rotated away. Forgetting it here is the honest end of that story.
			_ = u.forgetAccount(ctx, stored.Server)
			return "", domain.Refuse(domain.CodeNotSignedIn, domain.ErrNotAllowed, nil)
		}
		return "", err
	}

	u.mu.Lock()
	u.access, u.expiresAt = tokens.Access, tokens.ExpiresAt
	u.mu.Unlock()

	if tokens.Refresh != "" && tokens.Refresh != refresh {
		if err := u.store.Save(ctx, stored, tokens.Refresh); err != nil {
			return "", err
		}
	}
	return tokens.Access, nil
}

// forgetAccount drops the account and the token that kept it, and leaves the address: it is where
// the app would sign in again, and it is a choice rather than a credential.
func (u *UseCase) forgetAccount(ctx context.Context, server string) error {
	u.forgetTokens()
	if err := u.store.Forget(ctx); err != nil {
		return err
	}
	if server == "" {
		return nil
	}
	return u.store.Save(ctx, domain.Account{Server: server}, "")
}

// announce tells every window what the account is now.
func (u *UseCase) announce(ctx context.Context) (State, error) {
	state, err := u.State(ctx)
	if err != nil {
		return State{}, err
	}
	u.notifier.Publish(TopicChanged, state)
	return state, nil
}

func (u *UseCase) forgetTokens() {
	u.mu.Lock()
	u.access, u.expiresAt = "", time.Time{}
	u.mu.Unlock()
}

// stop ends a watch that is in progress, if there is one.
func (u *UseCase) stop() {
	u.mu.Lock()
	cancel := u.cancel
	u.cancel = nil
	u.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// device is what this installation calls itself. The server cannot tell a laptop from a phone, and
// a name taken from the machine would be a name the person never chose — so the app says what it is
// and where it runs, and nothing more.
func (u *UseCase) device() domain.Device {
	return domain.Device{
		Name:       platform.Name,
		Platform:   platform.OSName(),
		AppVersion: platform.NewBuildInfo().Version,
	}
}

// serverAddress is the address calls go to: what a person chose, and the program's own server when
// they chose nothing. Every call that leaves here goes through it — the row keeps the choice alone,
// because a window draws a server of somebody's own and never the product's.
func serverAddress(chosen string) string {
	if named := chosenAddress(chosen); named != "" {
		return named
	}
	return platform.DefaultServer
}

// chosenAddress is what a person's answer means: their address with a scheme and without a trailing
// slash, or nothing at all when they named none. Nothing is an answer and not a mistake — it means
// the program's own server, which is why it is kept as nothing rather than written out.
func chosenAddress(address string) string {
	trimmed := strings.TrimSpace(address)
	if trimmed == "" {
		return ""
	}
	if !strings.Contains(trimmed, "://") {
		// A person types "app.example.com", not "https://app.example.com". Assuming the secure scheme
		// is the safe guess: the wrong one is a link that does not work, and this one keeps the token
		// out of the clear.
		trimmed = "https://" + trimmed
	}
	return strings.TrimSuffix(trimmed, "/")
}

// failureOf is the refusal as it will travel to the window: the code and the values its sentence
// needs. A failure nobody wrote a sentence for is said as the server being at fault — the app has
// no words of its own for a bug.
func failureOf(err error) *domain.Failure {
	if failure := domain.AsFailure(err); failure != nil {
		return failure
	}
	return &domain.Failure{Code: domain.CodeSignInFailed}
}
