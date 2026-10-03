package main

import (
	"strconv"
	"strings"
)

var (
	Version   = "1.0.1"
	BuildDate = ""
	GitCommit = ""
)

// CompareVersions compares two semver strings (e.g. "1.0.1" and "1.0.0").
// Returns 1 if v1 > v2, -1 if v1 < v2, and 0 if v1 == v2.
func CompareVersions(v1, v2 string) int {
	v1 = strings.TrimSpace(strings.TrimPrefix(v1, "v"))
	v2 = strings.TrimSpace(strings.TrimPrefix(v2, "v"))

	// Strip any build metadata or suffixes (e.g. "-beta", " (legacy)")
	if idx := strings.IndexAny(v1, "-+ "); idx != -1 {
		v1 = v1[:idx]
	}
	if idx := strings.IndexAny(v2, "-+ "); idx != -1 {
		v2 = v2[:idx]
	}

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			n1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			n2, _ = strconv.Atoi(parts2[i])
		}
		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}

	return 0
}
