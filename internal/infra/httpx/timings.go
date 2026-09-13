package httpx

import "time"

// diffUs reports how long a phase took, or nothing when there was no phase to time.
//
// Microseconds rather than milliseconds is the difference between a breakdown and a row of zeroes:
// over a connection that is already open the body of a small response arrives in a fraction of a
// millisecond, and a phase rounded to zero reads as one that failed to be measured. Nothing is what
// a missing pair means — a reused connection never dials, so the connect event never fires — and
// that is a different fact from a phase that was measured and took no time.
func diffUs(start, end time.Time) *int64 {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return nil
	}
	us := end.Sub(start).Microseconds()
	return &us
}
