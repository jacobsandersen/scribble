package util

import "strings"

// NormalizeBaseURL ensures the base URL ends with a slash.
func NormalizeBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimRight(trimmed, "/")
	return trimmed + "/"
}

// DeriveTableName constructs the content table name from the configured prefix.
// If no prefix is set, defaults to "scribble"; empty string produces "content".
func DeriveTableName(prefix *string) string {
	p := "scribble"
	if prefix != nil {
		p = *prefix
	}

	if p == "" {
		return "content"
	}

	return p + "_content"
}
