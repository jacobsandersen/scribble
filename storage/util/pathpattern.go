package util

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// PathPattern represents a configurable pattern for generating file paths.
// It supports placeholders that get replaced with actual values:
//   - {year}    - 4-digit year (e.g., "2026")
//   - {month}   - 2-digit month (e.g., "01")
//   - {day}     - 2-digit day (e.g., "15")
//   - {slug}    - the document slug
//   - {ext}     - file extension (with leading dot, e.g., ".json")
//   - {filename} - full filename including extension
//
// Example patterns:
//   - "{year}/{month}/{slug}.json" → "2026/01/my-post.json"
//   - "posts/{slug}{ext}" → "posts/my-post.json"
//   - "{year}/{month}/{day}/{filename}" → "2026/01/15/my-post.json"
type PathPattern struct {
	pattern string
}

// NewPathPattern creates a new PathPattern from a template string.
func NewPathPattern(pattern string) *PathPattern {
	return &PathPattern{pattern: pattern}
}

// Generate produces a file path by replacing placeholders with actual values.
// The slug parameter is required. The timestamp is optional (pass time.Time{}
// to skip date-based placeholders). The extension is optional (pass empty string
// to skip).
func (p *PathPattern) Generate(slug string, timestamp time.Time, ext string) (string, error) {
	if slug == "" {
		return "", fmt.Errorf("slug cannot be empty")
	}

	result := p.pattern

	// Replace date placeholders if timestamp is provided
	if !timestamp.IsZero() {
		result = strings.ReplaceAll(result, "{year}", fmt.Sprintf("%04d", timestamp.Year()))
		result = strings.ReplaceAll(result, "{month}", fmt.Sprintf("%02d", timestamp.Month()))
		result = strings.ReplaceAll(result, "{day}", fmt.Sprintf("%02d", timestamp.Day()))
	}

	// Ensure extension has leading dot if provided
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	// Build filename
	filename := slug
	if ext != "" {
		filename = slug + ext
	}

	// Replace slug and filename placeholders
	result = strings.ReplaceAll(result, "{slug}", slug)
	result = strings.ReplaceAll(result, "{filename}", filename)
	result = strings.ReplaceAll(result, "{ext}", ext)

	// Clean the path (removes double slashes, etc.)
	result = filepath.Clean(result)

	return result, nil
}

// DefaultContentPattern returns the default pattern for content files.
// Pattern: "{slug}.json" (flat structure in content directory)
func DefaultContentPattern() *PathPattern {
	return NewPathPattern("{slug}.json")
}

// DefaultMediaPattern returns the default pattern for media files.
// Pattern: "{year}/{month}/{filename}" (organized by date)
func DefaultMediaPattern() *PathPattern {
	return NewPathPattern("{year}/{month}/{filename}")
}
