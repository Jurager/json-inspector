package domain

import (
	"errors"
	"sort"
	"strings"
)

// Code names a failure the window can word itself. It is not a message: the catalogue holds one
// sentence per code per language, and the window picks it — which is why the app can refuse in words
// it does not know.
//
// A code travels to the window beside the error, not inside it: Wails marshals a rejected call's
// error through the app's own marshaller, and the window reads the code off that. The error's own
// text stays what it always was — the machine's account, for the log and for a bug report.
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
	CodePersonalWorkspace  Code = "personalWorkspace"
	CodeWorkspaceMissing   Code = "workspaceMissing"
)

// Args are the values a code's sentence interpolates. They are strings because that is what a message
// interpolates, and because a value that is a number is formatted where it is known to be one — a file
// limit is counted in the unit its sentence names, not in bytes.
//
// The same values make the error's own text, so an arg is read twice: by the sentence, which uses the
// ones it names, and by a log, which prints all of them.
type Args map[string]string

// Failure is an error the window can word itself: a code, the values its sentence needs, and the
// sentinel a caller above asks `errors.Is` about.
//
// It marshals to exactly what the window needs and nothing more, which is what makes it worth a type
// of its own rather than a formatted string.
type Failure struct {
	Code Code `json:"code"`
	Args Args `json:"args,omitempty"`

	sentinel error
}

// Error is the machine's account of the refusal: the code, and the values it was given. It is stable
// and greppable, which is what a log wants, and it is never shown as if it were the app's own words.
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
// the values the sentence needs. A nil sentinel is a refusal that maps onto none of the domain's own —
// it will not answer `errors.Is`, and only the code will describe it.
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

// CodeOf names what an error is. An error the app did not build has no code, and the window shows the
// machine's message for it — which is the honest thing to do with a failure nobody has words for.
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
