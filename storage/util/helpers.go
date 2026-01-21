package util

import "strings"

// NormalizeBaseURL ensures the base URL ends with a slash.
func NormalizeBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimRight(trimmed, "/")
	return trimmed + "/"
}
