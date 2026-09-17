package domain

import "time"

// The account this installation is signed in as, and the devices it is signed in on. It is the one
// part of the app that is not about this machine: everything else — collections, environments,
// history — lives here and stays here, and the account is what the app knows about the other side.
//
// The server is the authority on all of it. What is kept here is the last answer it gave, so that
// the window can draw the account without asking, and so that a device that is offline still says
// who it belongs to.

// PlanFree is what a server that sells nothing answers. The app draws no tariff block for it: a
// plan nobody can buy is not a thing to offer somebody.
const PlanFree = "free"

// Account is the account, as the server last described it.
type Account struct {
	// Server is the address a person named for a self-hosted deployment, and it is empty while the
	// app is on the server it ships with — the two are not the same thing, and only the first is
	// something a window draws. It is kept even when nobody is signed in: a person chose it once, and
	// choosing it twice is asking twice.
	Server string `json:"server"`
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	// Plan and Seats are what the account is entitled to. Both are empty on a server that answers
	// nothing about plans, which is what the app draws nothing for.
	Plan  string `json:"plan"`
	Seats int    `json:"seats"`
	// SessionID is this device's session on the server. The list of devices cannot say which row is
	// the one asking without it, and the server does not guess.
	SessionID  string    `json:"sessionId"`
	SignedInAt time.Time `json:"signedInAt"`
}

// SignedIn says whether there is an account behind this. The app is usable without one, and that is
// the whole of what this is for.
func (a Account) SignedIn() bool { return a.UserID != "" }

// HasPlan says whether the server told us about a plan worth drawing. A self-hosted server answers
// free, and free is not news.
func (a Account) HasPlan() bool { return a.Plan != "" && a.Plan != PlanFree }

// Device is what this installation calls itself when it signs in. All three are what the app knows
// about the machine it runs on: the server cannot tell a laptop from a phone, and a guess from a
// user agent is a name the person would have to live with.
//
// The phrase a person reads in the list of devices — "Windows · JSON Inspector 2.3.6" — is put
// together where it is drawn. A method here would not reach the window: the bindings carry these as
// plain data, and a joined string in the server's answer would be a second copy of the same three
// fields.
type Device struct {
	Name       string `json:"name"`
	Platform   string `json:"platform"`
	AppVersion string `json:"appVersion"`
}

// Session is one device the account is signed in on, as the server reports it. The server knows
// more than this — a client id, the scopes it was granted — and none of it is anything a person
// reads.
type Session struct {
	ID     string `json:"id"`
	Device Device `json:"device"`
	// IP is where the server saw the device sign in, as far as it can tell. The city the design draws
	// beside it is not here: the server has no source of geodata, and a guess is worse than nothing.
	IP         string    `json:"ip,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	LastSeenAt time.Time `json:"lastSeenAt"`
}

// Tokens is what a server hands a device: the short-lived credential every call carries, and the
// long-lived one that buys the next.
//
// It is a domain type rather than one of the use case's, and that is not a detail: the client that
// speaks to the server implements a port declared by the use case, and a port whose answers are
// types of the use case would make the two import each other.
type Tokens struct {
	Access  string
	Refresh string
	// ExpiresAt is when the access token stops working. The refresh token's own life is the server's
	// business — the app finds out by using it.
	ExpiresAt time.Time
}

// Challenge is a device code the person has to confirm: what the app shows them, and where they
// confirm it. The code is short because a person reads it off one screen and checks it on another.
type Challenge struct {
	UserCode  string    `json:"userCode"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expiresAt"`
}
