package wails

import (
	"encoding/json"

	"json-inspector/internal/domain"
)

// marshalFailure is what a rejected call carries beside its message: the code and the values the
// window words itself. Wails hands every error a bound method returned to this before it crosses,
// and the window reads the answer off `error.cause`.
//
// Returning nothing is not a failure: the option falls back to Wails' own marshaller, and the
// window shows the machine's message. An error the app did not build — a disk that failed, a socket
// that closed — has no code, and inventing one would be worse than saying so.
func marshalFailure(err error) []byte {
	code := domain.CodeOf(err)
	if code == "" {
		return nil
	}

	failure := domain.Failure{Code: code}
	if found := domain.AsFailure(err); found != nil {
		failure = *found
	}
	encoded, marshalErr := json.Marshal(failure)
	if marshalErr != nil {
		return nil
	}
	return encoded
}
