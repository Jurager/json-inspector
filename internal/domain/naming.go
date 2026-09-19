package domain

import (
	"strconv"
	"strings"
)

// CleanName is a name something is given: trimmed, non-empty, and inside the ceiling that kind of
// thing allows. The ceiling is the caller's, because it differs — a collection's name is a row of
// a tree, an environment's is a line of a settings sheet — while the two refusals are the
// catalogue's, so the window words them the same way wherever it draws one.
//
// It lives here rather than in each feature because three features had written it out identically,
// each with its own constant, and a rule that is copied is a rule that drifts.
func CleanName(raw string, max int) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", Refuse(CodeNameEmpty, ErrNotAllowed, nil)
	}
	if len([]rune(name)) > max {
		return "", Refuse(CodeNameTooLong, ErrNotAllowed, Args{"max": strconv.Itoa(max)})
	}
	return name, nil
}
