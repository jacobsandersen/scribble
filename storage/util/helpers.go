package util

import (
	"fmt"
	"strings"
)

// NormalizeBaseURL ensures the base URL ends with a slash.
func NormalizeBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimRight(trimmed, "/")
	return trimmed + "/"
}

// DeriveTableName constructs a scribble table name from the configured prefix, if any.
func DeriveTableName(prefix string, table string) string {
	if prefix == "" {
		return table
	}

	return fmt.Sprintf("%s_%s", prefix, table)
}
