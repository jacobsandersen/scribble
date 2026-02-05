package util

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/indieinfra/scribble/server/util"
)

type PathPattern struct {
	pattern string
}

const slugParam = "{slug}"

// NewPathPattern creates a new PathPattern from a template string.
func NewContentPathPattern(pattern string) (*PathPattern, error) {
	if !strings.Contains(pattern, slugParam) {
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
	result = strings.ReplaceAll(result, slugParam, slug)

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

func (p *PathPattern) ReplaceSlugParam(urlInst string, slug string) (string, error) {
	if slug == "" {
		return "", fmt.Errorf("slug cannot be empty")
	}

	if !util.UrlIsInstance(p.pattern, urlInst) {
		return "", fmt.Errorf("URL does not match the path pattern")
	}

	// Reuse existing helpers to parse and normalize URLs/paths.
	patternPath, err := util.GetUrlPath(strings.ReplaceAll(p.pattern, "\\", "/"))
	if err != nil {
		return "", fmt.Errorf("invalid path pattern: %w", err)
	}

	targetPath, err := util.GetUrlPath(strings.ReplaceAll(urlInst, "\\", "/"))
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	patternSegments := filterEmpty(strings.Split(patternPath, "/"))
	targetSegments := filterEmpty(strings.Split(targetPath, "/"))

	slugIdx := -1
	for i, seg := range patternSegments {
		if seg == slugParam {
			slugIdx = i
			break
		}
	}

	if slugIdx == -1 || slugIdx >= len(targetSegments) {
		return "", fmt.Errorf("pattern path is missing {slug} placeholder")
	}

	targetSegments[slugIdx] = slug

	targetURL, err := url.Parse(strings.ReplaceAll(urlInst, "\\", "/"))
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	targetURL.Path = "/" + strings.Join(targetSegments, "/")

	return targetURL.String(), nil
}

func filterEmpty(parts []string) []string {
	if len(parts) == 0 {
		return parts
	}

	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
