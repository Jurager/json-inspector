package platform

import "github.com/google/uuid"

// IDGen generates ids for IPC and database records.
type IDGen func() string

func NewIDGen() IDGen {
	return newID
}

// newID returns a UUID v7, providing time-ordered ids.
func newID() string {
	id, err := uuid.NewV7()
	if err != nil {
		// Fall back to UUID v4 if UUID v7 generation fails.
		return uuid.NewString()
	}

	return id.String()
}
