package account

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

// The account is one row and a conversation with a server, and both halves are faked here: what
// these tests are about is what the app decides — when it asks, what it keeps, and what it says
// when the other side is not there.

// fakeStore is the row, in a map of one.
type fakeStore struct {
	mu     sync.Mutex
	row    domain.Account
	exists bool
	token  string
}

func newFakeStore() *fakeStore { return &fakeStore{} }

func (f *fakeStore) Account(context.Context) (domain.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.exists {
		return domain.Account{}, domain.ErrNotFound
	}
	return f.row, nil
}

// Save keeps the address when the account leaves, and the token when one is given: an empty token
// means "do not touch it", which is what changing the address does.
func (f *fakeStore) Save(_ context.Context, account domain.Account, refresh string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.row, f.exists = account, true
	if refresh != "" {
		f.token = refresh
	}
	if account.UserID == "" {
		f.token = ""
	}
	return nil
}

func (f *fakeStore) RefreshToken(context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.token == "" {
		return "", domain.ErrNotFound
	}
	return f.token, nil
}

func (f *fakeStore) Forget(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.row, f.exists, f.token = domain.Account{}, false, ""
	return nil
}

func (f *fakeStore) saved() (domain.Account, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.row, f.token
}

// fakeServer is the other side: what it answers, and when.
type fakeServer struct {
	mu    sync.Mutex
	calls []string
	// authorizeTo is the address the sign-in was asked to go to. A method name alone would not catch
	// a sign-in that went to the wrong server, and the address is what the app then lives on.
	authorizeTo string

	challenge    domain.Challenge
	authorizeErr error
	// confirming is where a test says what the person did: the answer the token endpoint would give.
	confirming chan answer

	me       domain.Account
	meErr    error
	sessions []domain.Session
	refresh  domain.Tokens
	// refreshErr is what the token endpoint says when the refresh token is no good.
	refreshErr error
}

type answer struct {
	tokens domain.Tokens
	err    error
}

func newFakeServer() *fakeServer {
	return &fakeServer{
		challenge: domain.Challenge{
			UserCode:  "BCDF-GHJK",
			URL:       "https://api.example.com/activate?user_code=BCDF-GHJK",
			ExpiresAt: time.Now().Add(10 * time.Minute),
		},
		confirming: make(chan answer, 1),
		me: domain.Account{
			UserID:    "user-1",
			Email:     "ucell.dev@itl.digital",
			Plan:      "free",
			Seats:     1,
			SessionID: "session-1",
		},
		sessions: []domain.Session{{ID: "session-1", Device: domain.Device{Platform: "macOS"}}},
		refresh:  tokens("access-2", "refresh-2"),
	}
}

func (f *fakeServer) note(method string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, method)
}

func (f *fakeServer) called() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.calls...)
}

func (f *fakeServer) Authorize(_ context.Context, address string, _ domain.Device) (
	domain.Challenge, error) {
	f.note("authorize")

	f.mu.Lock()
	f.authorizeTo = address
	f.mu.Unlock()

	return f.challenge, f.authorizeErr
}

// askedTo is where the sign-in went, which is the address the app is then signed in to.
func (f *fakeServer) askedTo() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.authorizeTo
}

// Tokens waits for the person, which is what the real one does: it returns when the test says what
// they answered, or when the app stops caring.
func (f *fakeServer) Tokens(ctx context.Context, _ string, _ domain.Challenge) (
	domain.Tokens, error) {
	f.note("tokens")
	select {
	case given := <-f.confirming:
		return given.tokens, given.err
	case <-ctx.Done():
		return domain.Tokens{}, ctx.Err()
	}
}

func (f *fakeServer) Refresh(context.Context, string, string) (domain.Tokens, error) {
	f.note("refresh")
	return f.refresh, f.refreshErr
}

func (f *fakeServer) Me(context.Context, string, string) (domain.Account, error) {
	f.note("me")
	return f.me, f.meErr
}

func (f *fakeServer) Sessions(context.Context, string, string) ([]domain.Session, error) {
	f.note("sessions")
	return f.sessions, nil
}

func (f *fakeServer) EndSession(_ context.Context, _, _, sessionID string) (
	[]domain.Session, error) {
	f.note("end " + sessionID)
	return f.sessions, nil
}

func (f *fakeServer) EndOthers(context.Context, string, string) ([]domain.Session, error) {
	f.note("end others")
	return f.sessions, nil
}

func (f *fakeServer) SignOut(context.Context, string, string) error {
	f.note("sign out")
	return nil
}

func (f *fakeServer) DeleteAccount(context.Context, string, string) error {
	f.note("delete")
	return nil
}

// fakeBrowser remembers what it was asked to open, and says nothing about whether it managed.
type fakeBrowser struct{ opened []string }

func (f *fakeBrowser) OpenURL(url string) error {
	f.opened = append(f.opened, url)
	return nil
}

// fakeNotifier hands every announcement to the test, which is what the window does.
type fakeNotifier struct{ events chan State }

func newFakeNotifier() *fakeNotifier { return &fakeNotifier{events: make(chan State, 8)} }

func (n *fakeNotifier) Publish(_ string, payload any) {
	state, ok := payload.(State)
	if !ok {
		return
	}
	n.events <- state
}

// until waits for the announcement a test is about. The app says several things while a sign-in
// runs its course — the code is out, the code was confirmed — and a test that read the first one
// would be reading the wrong step.
func (n *fakeNotifier) until(t *testing.T, wanted func(State) bool) State {
	t.Helper()

	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		select {
		case state := <-n.events:
			if wanted(state) {
				return state
			}
		case <-time.After(50 * time.Millisecond):
		}
	}
	t.Fatal("the announcement a test was waiting for never came")
	return State{}
}

func newUseCase() (*UseCase, *fakeStore, *fakeServer, *fakeBrowser, *fakeNotifier) {
	store, server := newFakeStore(), newFakeServer()
	browser, notifier := &fakeBrowser{}, newFakeNotifier()
	return NewUseCase(store, server, browser, notifier), store, server, browser, notifier
}

// tokens is the pair a confirmed code produces, good for an hour: what the app keeps in memory and
// what it hands to the server.
func tokens(access, refresh string) domain.Tokens {
	return domain.Tokens{Access: access, Refresh: refresh, ExpiresAt: time.Now().Add(time.Hour)}
}

// signedIn is an app that already has an account, which is where the tests about anything but
// signing in start. It waits for the sign-in to finish rather than assuming it has: the work
// happens in the watch, and a test that raced it would be a test about timing.
func signedIn(t *testing.T, uc *UseCase) {
	t.Helper()

	ctx := context.Background()
	notifier, ok := uc.notifier.(*fakeNotifier)
	if !ok {
		t.Fatal("the use case was built without the fake notifier")
	}

	if _, err := uc.Begin(ctx, "https://api.example.com"); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	theServer(t, uc).confirming <- answer{
		tokens: tokens("access-1", "refresh-1"),
	}
	notifier.until(t, func(state State) bool { return state.SignedIn })
}

// expired moves the app's clock past the token it holds, which is what makes it go and buy another.
func expired(uc *UseCase) {
	uc.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
}

// theServer is the fake under the wrapper the use case talks to: every call to the port is recorded
// by the wrapper, so a test driving the fake has to reach through it.
func theServer(t *testing.T, uc *UseCase) *fakeServer {
	t.Helper()

	watcher, ok := uc.server.(watched)
	if !ok {
		t.Fatal("the use case was built without the wrapper that records reachability")
	}
	server, ok := watcher.Server.(*fakeServer)
	if !ok {
		t.Fatal("the port under the wrapper is not the fake")
	}
	return server
}

func TestASignInShowsACodeAndOpensTheBrowser(t *testing.T) {
	uc, _, _, browser, _ := newUseCase()

	state, err := uc.Begin(context.Background(), "https://api.example.com")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}

	if state.Challenge.UserCode != "BCDF-GHJK" {
		t.Errorf("code = %q, want the one the server handed out", state.Challenge.UserCode)
	}
	if !state.Waiting {
		t.Error("the app is not waiting for anybody, and the person has not answered yet")
	}
	// The browser opens on the address the server gave, code and all: the person should not have to
	// type anything.
	if len(browser.opened) != 1 || browser.opened[0] != state.Challenge.URL {
		t.Errorf("opened %v, want the confirmation page", browser.opened)
	}
}

func TestASignInThatIsConfirmedEndsWithAnAccount(t *testing.T) {
	uc, store, server, _, notifier := newUseCase()

	if _, err := uc.Begin(context.Background(), "api.example.com"); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	server.confirming <- answer{
		tokens: tokens("access-1", "refresh-1"),
	}

	announced := notifier.until(t, func(state State) bool { return state.SignedIn })
	if announced.Account.Email != "ucell.dev@itl.digital" {
		t.Errorf("email = %q, want the address the server answered with", announced.Account.Email)
	}
	if announced.Account.SessionID != "session-1" {
		t.Errorf("session = %q, want the one the server named", announced.Account.SessionID)
	}
	// The address is kept with the scheme it was given: the app talks to this one again.
	if announced.Account.Server != "https://api.example.com" {
		t.Errorf("server = %q, want the normalised address", announced.Account.Server)
	}
	if announced.Waiting {
		t.Error("the app is still waiting after the code was confirmed")
	}

	saved, token := store.saved()
	if saved.UserID != "user-1" || token != "refresh-1" {
		t.Errorf("kept %+v with token %q, want the account and its credential", saved, token)
	}
}

func TestASignInThatCameToNothingSaysSo(t *testing.T) {
	uc, _, server, _, notifier := newUseCase()

	if _, err := uc.Begin(context.Background(), "https://api.example.com"); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	// The code ran out while the person was looking for their phone.
	server.confirming <- answer{err: errors.New("expired")}

	announced := notifier.until(t, func(state State) bool { return state.Failure != nil })
	if announced.Failure == nil {
		t.Fatal("the app said nothing about a sign-in that went nowhere")
	}
	if announced.SignedIn || announced.Waiting {
		t.Errorf("state = %+v, want nothing signed in and nothing in progress", announced)
	}
}

func TestCancellingASignInStopsWaiting(t *testing.T) {
	uc, _, server, _, notifier := newUseCase()

	if _, err := uc.Begin(context.Background(), "https://api.example.com"); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if _, err := uc.Cancel(context.Background()); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	announced := notifier.until(t, func(state State) bool { return !state.Waiting })
	if announced.Waiting {
		t.Errorf("state = %+v, want the wait to be over", announced)
	}
	// Nothing else is announced afterwards: a code confirmed after the person gave up must not sign
	// anybody in.
	server.confirming <- answer{
		tokens: tokens("access-1", "refresh-1"),
	}

	select {
	case late := <-notifier.events:
		if late.SignedIn {
			t.Fatalf("a cancelled sign-in signed the app in: %+v", late)
		}
	case <-time.After(200 * time.Millisecond):
	}
}

func TestTheAddressIsKeptWithASchemeAndWithoutASlash(t *testing.T) {
	uc, store, _, _, _ := newUseCase()

	if _, err := uc.SetServer(context.Background(), "  api.example.com/  "); err != nil {
		t.Fatalf("SetServer: %v", err)
	}
	saved, _ := store.saved()
	if saved.Server != "https://api.example.com" {
		t.Errorf("server = %q, want a scheme and no trailing slash", saved.Server)
	}
}

// An installation nobody has told about a server is not one that cannot sign in: the program has
// one of its own, so a person who has never heard of self-hosting presses the button and is given
// a code.
func TestASignInWithNoAddressGoesToTheProgramsOwnServer(t *testing.T) {
	uc, store, server, _, _ := newUseCase()

	state, err := uc.Begin(context.Background(), "")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if went := server.askedTo(); went != platform.DefaultServer {
		t.Errorf("the sign-in went to %q, want %q", went, platform.DefaultServer)
	}
	// Nothing was chosen, so nothing is drawn: the window shows an address only for a self-hosted
	// deployment, and this installation never named one.
	if state.Account.Server != "" {
		t.Errorf("the window is shown %q, want no server of its own", state.Account.Server)
	}
	saved, _ := store.saved()
	if saved.Server != "" {
		t.Errorf("stored server = %q, want nothing kept", saved.Server)
	}
}

// And a sign-in that goes to a server somebody named keeps the name: that is a self-hosted
// installation, and the window says which server it is on.
func TestASignInToAChosenServerKeepsTheName(t *testing.T) {
	uc, store, server, _, notifier := newUseCase()

	if _, err := uc.Begin(context.Background(), "api.example.com"); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	server.confirming <- answer{tokens: tokens("access-1", "refresh-1")}
	announced := notifier.until(t, func(state State) bool { return state.SignedIn })

	if went := server.askedTo(); went != "https://api.example.com" {
		t.Errorf("the sign-in went to %q, want the named server", went)
	}
	if announced.Account.Server != "https://api.example.com" {
		t.Errorf("the window is shown %q, want the named server", announced.Account.Server)
	}
	saved, _ := store.saved()
	if saved.Server != "https://api.example.com" {
		t.Errorf("stored server = %q, want the named server", saved.Server)
	}
}

// Clearing the address is an answer and not a mistake — "the program's own server" — and what is
// kept is the clearing itself, so that a build pointing somewhere else later takes this
// installation with it. The window then has no server of its own to draw, which is the point.
func TestClearingTheAddressGoesBackToTheProgramsOwnServer(t *testing.T) {
	uc, store, _, _, _ := newUseCase()

	if _, err := uc.SetServer(context.Background(), "api.example.com"); err != nil {
		t.Fatalf("SetServer: %v", err)
	}
	state, err := uc.SetServer(context.Background(), "   ")
	if err != nil {
		t.Fatalf("SetServer: %v", err)
	}
	if state.Account.Server != "" {
		t.Errorf("the window is shown %q, want no server of its own", state.Account.Server)
	}
	saved, _ := store.saved()
	if saved.Server != "" {
		t.Errorf("stored server = %q, want nothing kept", saved.Server)
	}
}

func TestSigningOutForgetsTheAccountAndKeepsTheAddress(t *testing.T) {
	uc, store, server, _, _ := newUseCase()
	signedIn(t, uc)

	state, err := uc.SignOut(context.Background())
	if err != nil {
		t.Fatalf("SignOut: %v", err)
	}
	if state.SignedIn {
		t.Error("the app is still signed in after signing out")
	}

	saved, token := store.saved()
	if saved.Server != "https://api.example.com" {
		t.Errorf("server = %q, want the address kept for next time", saved.Server)
	}
	if saved.UserID != "" || token != "" {
		t.Errorf("kept %+v with token %q, want nothing of the account", saved, token)
	}
	if !contains(server.called(), "sign out") {
		t.Errorf("the server was not told: %v", server.called())
	}
}

func TestLeavingWorksEvenWhenTheServerIsAway(t *testing.T) {
	uc, store, server, _, _ := newUseCase()
	signedIn(t, uc)

	server.refreshErr = domain.Refuse(domain.CodeServerUnreachable, nil, nil)
	expired(uc)

	if _, err := uc.SignOut(context.Background()); err != nil {
		t.Fatalf("SignOut: %v", err)
	}
	// The session stays on the server — nobody could tell it — and that is not a reason to keep
	// somebody signed in on a machine they asked to leave.
	if contains(server.called(), "sign out") {
		t.Errorf("called: %v, want no call to a server that is away", server.called())
	}
	saved, token := store.saved()
	if saved.SignedIn() || token != "" {
		t.Errorf("kept %+v with token %q, want the account gone", saved, token)
	}
}

func TestAServerThatCannotBeReachedIsNotALostAccount(t *testing.T) {
	uc, store, server, _, _ := newUseCase()
	signedIn(t, uc)

	server.refreshErr = domain.Refuse(domain.CodeServerUnreachable, nil, nil)
	expired(uc)

	_, err := uc.Sessions(context.Background())
	if domain.CodeOf(err) != domain.CodeServerUnreachable {
		t.Fatalf("Sessions = %v, want the server being out of reach", err)
	}
	// The account is still here: a server that is down is weather, not a reason to sign somebody out.
	saved, _ := store.saved()
	if saved.UserID != "user-1" {
		t.Errorf("kept %+v, want the account left alone", saved)
	}
}

func TestACheckOnAServerThatIsAwaySaysSo(t *testing.T) {
	uc, store, server, _, _ := newUseCase()
	signedIn(t, uc)

	// Once while it is there, so that the moment it last answered is known and can be watched.
	answered, err := uc.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	server.refreshErr = domain.Refuse(domain.CodeServerUnreachable, nil, nil)
	expired(uc)

	state, err := uc.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	// The verdict travels as state rather than as an error: a server that does not answer has
	// answered this question, and a call that failed would leave the window with nothing to draw.
	if !state.Reachability.Checked || state.Reachability.Reachable {
		t.Errorf("reachability = %+v, want checked and out of reach", state.Reachability)
	}
	// «Последняя связь» is when it last answered, and a call that never got there does not move it.
	if !state.Reachability.At.Equal(answered.Reachability.At) {
		t.Errorf("kept %v, want the moment the server last answered (%v)",
			state.Reachability.At, answered.Reachability.At)
	}
	saved, _ := store.saved()
	if saved.UserID != "user-1" {
		t.Errorf("kept %+v, want the account left alone", saved)
	}
}

func TestACheckOnAServerThatAnswersSaysSo(t *testing.T) {
	uc, _, _, _, _ := newUseCase()
	signedIn(t, uc)

	state, err := uc.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !state.Reachability.Checked || !state.Reachability.Reachable {
		t.Errorf("reachability = %+v, want checked and reached", state.Reachability)
	}
	if state.Reachability.At.IsZero() {
		t.Error("no moment was kept for a call that was made")
	}
}

func TestAServerThatRefusedStillCountsAsReached(t *testing.T) {
	uc, _, server, _, _ := newUseCase()
	signedIn(t, uc)

	// The server answers and says no: it is there, and "there" is the whole of what this asks. The
	// two are told apart because the person's next move is different in each case.
	server.meErr = domain.Refuse(domain.CodeServerRefused, nil, nil)

	state, err := uc.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !state.Reachability.Reachable {
		t.Errorf("reachability = %+v, want a server that answered", state.Reachability)
	}
}

func TestAnAccountNobodySignedIntoIsNotChecked(t *testing.T) {
	uc, _, server, _, _ := newUseCase()

	state, err := uc.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if state.Reachability.Checked {
		t.Errorf("reachability = %+v, want nothing said about a server nobody asked", state.Reachability)
	}
	if called := server.called(); len(called) != 0 {
		t.Errorf("called the server: %v, want no call at all", called)
	}
}

func TestACheckIsAnnouncedToEveryWindow(t *testing.T) {
	uc, _, server, _, notifier := newUseCase()
	signedIn(t, uc)

	server.refreshErr = domain.Refuse(domain.CodeServerUnreachable, nil, nil)
	expired(uc)

	if _, err := uc.Check(context.Background()); err != nil {
		t.Fatalf("Check: %v", err)
	}

	// Every window draws the account, so the verdict is published rather than only handed back to
	// whoever asked: the window that asked is not the only one looking.
	arrived := notifier.until(t, func(state State) bool {
		return state.Reachability.Checked && !state.Reachability.Reachable
	})
	if arrived.Reachability.Reachable {
		t.Errorf("announced %+v, want the server out of reach", arrived.Reachability)
	}
}

func TestARefusedRefreshEndsTheAccountHere(t *testing.T) {
	uc, store, server, _, _ := newUseCase()
	signedIn(t, uc)

	// The server says this refresh token is no good: the account was ended somewhere else.
	server.refreshErr = domain.Refuse(domain.CodeServerRefused, nil, nil)
	expired(uc)

	_, err := uc.Sessions(context.Background())
	if domain.CodeOf(err) != domain.CodeNotSignedIn {
		t.Fatalf("Sessions = %v, want the app to say nobody is signed in", err)
	}
	saved, token := store.saved()
	if saved.UserID != "" || token != "" {
		t.Errorf("kept %+v with token %q, want the dead account dropped", saved, token)
	}
}

func TestTheTokenIsBoughtOnceAndKeptUntilItRunsOut(t *testing.T) {
	uc, _, server, _, _ := newUseCase()
	signedIn(t, uc)

	// The token from the sign-in is good for an hour, and asking three times does not buy three of
	// them: what a call needs is a token, not a new one.
	for range 3 {
		if _, err := uc.Sessions(context.Background()); err != nil {
			t.Fatalf("Sessions: %v", err)
		}
	}
	if bought(server) != 0 {
		t.Errorf("the token was bought %d times while it was still good", bought(server))
	}

	// And when it has run out, one call buys the next one — and the two after it do not.
	//
	// The renewed token's own lifetime is measured from the app's clock, which is the one the app
	// looks at: the fixture's hour is the hour it would really have been given.
	expired(uc)
	server.refresh = domain.Tokens{
		Access: "access-2", Refresh: "refresh-2", ExpiresAt: uc.now().Add(time.Hour),
	}
	for range 3 {
		if _, err := uc.Sessions(context.Background()); err != nil {
			t.Fatalf("Sessions: %v", err)
		}
	}
	if bought(server) != 1 {
		t.Errorf("the token was bought %d times, want the one renewal it needed", bought(server))
	}
}

// bought is how many times the app asked the server for a token.
func bought(server *fakeServer) int {
	count := 0
	for _, call := range server.called() {
		if call == "refresh" {
			count++
		}
	}
	return count
}

func TestEndingThisDeviceSignsTheAppOut(t *testing.T) {
	uc, store, _, _, _ := newUseCase()
	signedIn(t, uc)

	if _, err := uc.EndSession(context.Background(), "session-1"); err != nil {
		t.Fatalf("EndSession: %v", err)
	}

	saved, token := store.saved()
	if saved.UserID != "" || token != "" {
		t.Errorf("kept %+v with token %q, want the app signed out of a session that is gone",
			saved, token)
	}
}

func TestEndingAnotherDeviceLeavesThisOneAlone(t *testing.T) {
	uc, store, _, _, _ := newUseCase()
	signedIn(t, uc)

	if _, err := uc.EndSession(context.Background(), "session-2"); err != nil {
		t.Fatalf("EndSession: %v", err)
	}

	saved, _ := store.saved()
	if saved.UserID != "user-1" {
		t.Errorf("kept %+v, want this device still signed in", saved)
	}
}

func TestLeavingForgetsTheAccountOnTheServerAndHere(t *testing.T) {
	uc, store, server, _, _ := newUseCase()
	signedIn(t, uc)

	if _, err := uc.DeleteAccount(context.Background()); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}

	if !contains(server.called(), "delete") {
		t.Errorf("the server was not told: %v", server.called())
	}
	saved, _ := store.saved()
	if saved.UserID != "" {
		t.Errorf("kept %+v, want the account gone from here too", saved)
	}
}

func contains(all []string, wanted string) bool {
	for _, one := range all {
		if strings.HasPrefix(one, wanted) {
			return true
		}
	}
	return false
}
