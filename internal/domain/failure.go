package domain

import (
	"errors"
	"sort"
	"strings"
)

// Code names a failure the window can word itself: the catalogue holds one sentence per code per
// language, which is how the app can refuse in words it does not know.
//
// It travels beside the error rather than inside it — Wails marshals a rejected call's error
// through the app's own marshaller and the window reads the code off that — and the error's own
// text stays the machine's account, for the log and for a bug report.
type Code string

const (
	// The four the domain's own sentinels stand for. A site that wraps a bare sentinel gets one of
	// these without saying anything, which is why most call sites are untouched.
	CodeNotFound      Code = "notFound"
	CodeNotAllowed    Code = "notAllowed"
	CodeConflict      Code = "conflict"
	CodeStaleRevision Code = "staleRevision"

	// Where the reason a person needs is narrower than "it was refused".
	CodeNameEmpty          Code = "nameEmpty"
	CodeNameTooLong        Code = "nameTooLong"
	CodeDescriptionTooLong Code = "descriptionTooLong"
	CodeFileMissing        Code = "fileMissing"
	CodeFileUnreadable     Code = "fileUnreadable"
	CodeFileTooLarge       Code = "fileTooLarge"
	CodeNotJSON            Code = "notJson"
	CodeNotPostman         Code = "notPostmanCollection"
	CodeRunInProgress      Code = "runInProgress"
	CodeNodeHasNoRequests  Code = "nodeHasNoRequests"
	CodeNoEnvironment      Code = "noEnvironment"
	CodeUnknownField       Code = "unknownField"
	CodeUnknownList        Code = "unknownList"
	CodeDraftWithoutID     Code = "draftWithoutId"
	CodeIntoItself         Code = "intoItself"
	CodeUnknownTheme       Code = "unknownTheme"
	CodeUnknownLanguage    Code = "unknownLanguage"
	CodeUnknownListSide    Code = "unknownListSide"
	CodeUnknownRetention   Code = "unknownRetention"
	CodeUnknownChannel     Code = "unknownUpdateChannel"
	// The workspace the app is born with was the one that could not be deleted, and the rule is about
	// number now: the app has to have somewhere to keep its data, so the last space may not go —
	// whichever one it happens to be.
	CodeLastWorkspace    Code = "lastWorkspace"
	CodeWorkspaceMissing Code = "workspaceMissing"
	// A collection leaves the machine — it is exported, duplicated and handed on — so a secret has
	// no place in one, and this is the refusal that says so.
	CodeVariableSecret Code = "variableSecret"
	// An environment the user has closed to editing. The flag is a promise about the whole side, not
	// about the button somebody happened to press, so the write itself is what refuses.
	CodeEnvironmentReadOnly Code = "environmentReadOnly"
	// A request that names a `{{token}}` nothing answers. It is a refusal rather than a request with
	// braces in its address: the server would read `{{var3}}` as a path and answer something wrong,
	// and a run that reported that answer would be reporting a request nobody meant to send.
	CodeVariableMissing Code = "variableMissing"

	// The account. What can go wrong with the other side of the wire is narrower than it looks: the
	// server either answers, refuses, or is not there — and the three are told apart because the
	// person's next move is different in each case.
	//
	// CodeServerUnreachable is the one that must never be an exception or a hang: the app is a local
	// tool first, and a server that is down or a laptop that is offline is an ordinary state, not a
	// broken install.
	CodeNotSignedIn       Code = "notSignedIn"
	CodeServerUnreachable Code = "serverUnreachable"
	CodeServerRefused     Code = "serverRefused"
	CodeSessionUnknown    Code = "sessionUnknown"
	// A sign-in that ends in nothing: the code ran out, the person said no, or the app was closed
	// while the browser was open. The next move is the same in all three — try again.
	CodeSignInFailed Code = "signInFailed"
)

// Args are the values a code's sentence interpolates. Strings because that is what a message
// interpolates, and a number is formatted where it is known to be one — a file limit is counted
// in the unit its sentence names, not in bytes.
type Args map[string]string

// Failure is an error the window can word itself: a code, the values its sentence needs, and the
// sentinel a caller above asks errors.Is about. It marshals to exactly what the window needs.
type Failure struct {
	Code Code `json:"code"`
	Args Args `json:"args,omitempty"`

	sentinel error
}

// Error is the machine's account of the refusal: the code, and the values it was given. It is
// stable and greppable, which is what a log wants, and it is never shown as if it were the app's
// own words.
func (f *Failure) Error() string {
	if len(f.Args) == 0 {
		return string(f.Code)
	}
	keys := make([]string, 0, len(f.Args))
	for key := range f.Args {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+f.Args[key])
	}
	return string(f.Code) + "(" + strings.Join(parts, ", ") + ")"
}

func (f *Failure) Unwrap() error { return f.sentinel }

// Refuse is the app saying no: a code the window words, the sentinel a caller above recognises, and
// the values the sentence needs. A nil sentinel is a refusal that maps onto none of the domain's
// own — it will not answer `errors.Is`, and only the code will describe it.
func Refuse(code Code, sentinel error, args Args) error {
	return &Failure{Code: code, Args: args, sentinel: sentinel}
}

// AsFailure is the failure an error carries, if it carries one.
func AsFailure(err error) *Failure {
	var failure *Failure
	if errors.As(err, &failure) {
		return failure
	}
	return nil
}

// sentinelCodes gives the domain's own refusals a code, so a site that wraps a bare sentinel still
// tells the window what kind of refusal it was. A site with something more precise to say builds a
// Failure instead, and the failure wins — it is the nearer answer.
var sentinelCodes = []struct {
	err  error
	code Code
}{
	{ErrNotFound, CodeNotFound},
	{ErrNotAllowed, CodeNotAllowed},
	{ErrConflict, CodeConflict},
	{ErrStaleRevision, CodeStaleRevision},
}

// CodeOf names what an error is. An error the app did not build has no code, and the window shows
// the machine's message for it — which is the honest thing to do with a failure nobody has words
// for.
func CodeOf(err error) Code {
	if failure := AsFailure(err); failure != nil {
		return failure.Code
	}
	for _, entry := range sentinelCodes {
		if errors.Is(err, entry.err) {
			return entry.code
		}
	}
	return ""
}
