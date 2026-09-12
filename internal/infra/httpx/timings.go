package httpx

import "time"

// diffMs reports a phase length, treating a missing or backwards pair as no phase at all — a
// reused connection legitimately skips DNS and connect, and those show as 0 rather than as noise.
func diffMs(start, end time.Time) int64 {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return 0
	}
	return end.Sub(start).Milliseconds()
}
