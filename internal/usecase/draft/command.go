package draft

// testdata holds a corpus dumped from the TypeScript this was ported from, and it is still the
// oracle — a case that changes has to be looked at rather than re-pinned. fixtures_test.go says
// where this deliberately leaves JavaScript's whitespace and key-object-order rules behind.

import (
	"context"

	"json-inspector/internal/domain"
)

// CommandReason is why a command was recognised but could not be read. These cross to the window,
// which has a sentence for each, so the values are a contract with the locale files rather than a
// name for anything on this side.
type CommandReason string

const (
	ReasonNoURL                CommandReason = "no-url"
	ReasonBadQuotes            CommandReason = "bad-quotes"
	ReasonLeftover             CommandReason = "leftover"
	ReasonUnsupportedMultipart CommandReason = "unsupported-multipart"
)

// CommandKind is what reading the text came to.
type CommandKind string

const (
	KindNone  CommandKind = "none"
	KindOK    CommandKind = "ok"
	KindError CommandKind = "error"
)

// KindNone is "not a command at all, leave the field alone", which is what most pastes come to;
// KindError carries the reason the window phrases. A command that was read is a Seed — the same
// whole request "открыть в запросе" produces.
type CommandResult struct {
	Kind   CommandKind   `json:"kind"`
	Seed   Seed          `json:"seed"`
	Reason CommandReason `json:"reason"`
}

// Paste is a pasted command applied to the draft: what the reading came to, and the draft as it
// stands when there turned out to be something to replace it with. State is absent for the pastes
// that were not commands at all, which is most of them.
type Paste struct {
	Reading CommandResult `json:"reading"`
	State   *State        `json:"state,omitempty"`
}

// The parse and the draft are applied here rather than by the window, which would otherwise take
// the request apart only to hand the pieces straight back. The reading comes back either way: the
// window has a sentence for a command it could not read.
func (u *UseCase) PasteCommand(ctx context.Context, id domain.DraftID, text string) (Paste, error) {
	reading := ParseCommand(text)
	if reading.Kind != KindOK {
		return Paste{Reading: reading}, nil
	}
	state, err := u.Replace(ctx, id, reading.Seed)
	if err != nil {
		return Paste{Reading: reading}, err
	}
	return Paste{Reading: reading, State: &state}, nil
}

// ParseCommand reads pasted text as a curl command. Most pastes are not commands at all, and
// coming back with KindNone is what lets the window put them in the field untouched.
func ParseCommand(text string) CommandResult {
	normalized := normalizeInput(text)
	if trimSpace(normalized) == "" {
		return CommandResult{Kind: KindNone}
	}

	rest, ok := detect(normalized)
	if !ok {
		return CommandResult{Kind: KindNone}
	}

	tokens, tokOK := tokenize(rest)
	if !tokOK {
		return CommandResult{Kind: KindError, Reason: ReasonBadQuotes}
	}

	parsed, reason := fromCurl(tokens)
	if reason != "" {
		return CommandResult{Kind: KindError, Reason: reason}
	}

	seed := parsed.seed()
	// Two inputs the TS calls ok and hands over unusable: an empty URL pastes an empty field, an
	// empty method renders an empty chip. An ok result is a promise that a request can be built from
	// it, so both are answered here instead. Pinned, with its reasoning, by TestEveryOKResultIsUsable.
	if seed.Method == "" {
		seed.Method = "GET"
	}
	if seed.URL == "" {
		return CommandResult{Kind: KindError, Reason: ReasonNoURL}
	}
	return CommandResult{Kind: KindOK, Seed: seed}
}
