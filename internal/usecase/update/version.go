package update

import (
	"strconv"
	"strings"
)

// Comparing versions. The app is released by tag, so this is a comparison of dotted numbers rather
// than anything semver-shaped: a tag with a suffix or a fourth part is read as far as it can be and
// no further. Both halves have to parse — a version nobody can read is not a version to be told
// about, and guessing in that direction would offer an update that may not exist.

func parseVersion(v string) ([]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	if v == "" {
		return nil, false
	}
	parts := strings.Split(v, ".")
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil, false
		}
		nums = append(nums, n)
	}
	return nums, true
}

func isNewer(candidate, current string) bool {
	c, okC := parseVersion(candidate)
	cur, okCur := parseVersion(current)
	if !okC || !okCur {
		return false
	}
	for i := 0; i < len(c) || i < len(cur); i++ {
		x, y := 0, 0
		if i < len(c) {
			x = c[i]
		}
		if i < len(cur) {
			y = cur[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}
