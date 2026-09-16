package httpx

import "time"

// diffUs reports a phase's length, or nothing when there was no phase to time — a reused
// connection never dials, so the event never fires. Microseconds and not milliseconds: a phase
// rounded to zero reads as one that failed to be measured rather than as one that took no time.
func diffUs(start, end time.Time) *int64 {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return nil
	}
	us := end.Sub(start).Microseconds()
	return &us
}
