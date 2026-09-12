package platform

import "github.com/google/uuid"

// IDGen mints the ids that cross the IPC boundary and end up as primary keys.
type IDGen func() string

func NewIDGen() IDGen {
	return NewID
}

// NewID returns a uuid v7: time-ordered, so ids sort the way their rows were created and a
// primary key stays a usable ordering key.
func NewID() string {
	id, err := uuid.NewV7()
	if err != nil {
		// Only a broken randomness source lands here, and a v4 is a better answer than failing
		// an insert over id shape.
		return uuid.NewString()
	}
	return id.String()
}
