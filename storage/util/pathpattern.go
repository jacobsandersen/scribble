package util

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type PathPattern struct {
	pattern string
}

// NewPathPattern creates a new PathPattern from a template string.
func NewContentPathPattern(pattern string) (*PathPattern, error) {
	if !strings.Contains(pattern, "{slug}") {
		return nil, fmt.Errorf("path pattern must contain {slug} placeholder")
	}

	return &PathPattern{pattern: pattern}, nil
}

func NewMediaPathPattern(pattern string) (*PathPattern, error) {
	if !strings.Contains(pattern, "{uuid}") {
		return nil, fmt.Errorf("path pattern must contain {uuid} placeholder")
	}

	return &PathPattern{pattern: pattern}, nil
}

func (p *PathPattern) GenerateContent(time time.Time, slug string) (string, error) {
	if slug == "" {
		return "", fmt.Errorf("slug cannot be empty")
	}

	result := p.generateCommon(time)
	result = strings.ReplaceAll(result, "{slug}", slug)

	return strings.TrimSpace(strings.Trim(result, "/")), nil
}

func (p *PathPattern) GenerateMedia(time time.Time, ext string) string {
	result := p.generateCommon(time)
	result = strings.ReplaceAll(result, "{uuid}", uuid.New().String())
	result = strings.ReplaceAll(result, "{ext}", ext)

	return strings.TrimSpace(strings.Trim(result, "/"))
}

func (p *PathPattern) generateCommon(time time.Time) string {
	result := p.pattern
	result = strings.ReplaceAll(result, "{year}", fmt.Sprintf("%04d", time.Year()))
	result = strings.ReplaceAll(result, "{month}", fmt.Sprintf("%02d", time.Month()))
	result = strings.ReplaceAll(result, "{day}", fmt.Sprintf("%02d", time.Day()))
	return result
}
