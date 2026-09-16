package wails

import (
	"encoding/json"

	"json-inspector/internal/domain"
)

// marshalFailure is what a rejected call carries beside its message: the code and the values the
// window words itself, read off `error.cause`. Returning nothing is not a failure — Wails' own
// marshaller takes it, and an error the app did not build has no code worth inventing.
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
