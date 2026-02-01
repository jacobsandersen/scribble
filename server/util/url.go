package util

import (
	"errors"
	"net/url"
	"strings"
)

func UrlIsInstance(pattern string, needle string) bool {
	normalizedPattern := strings.ReplaceAll(pattern, "\\", "/")
	normalizedNeedle := strings.ReplaceAll(needle, "\\", "/")

	patternURL, err := url.Parse(normalizedPattern)
	if err != nil {
		return false
	}

	actualURL, err := url.Parse(normalizedNeedle)
	if err != nil {
		return false
	}

	if !strings.EqualFold(patternURL.Scheme, actualURL.Scheme) {
		return false
	}

	if !strings.EqualFold(patternURL.Host, actualURL.Host) {
		return false
	}

	patternSegments := pathSegments(patternURL.Path)
	actualSegments := pathSegments(actualURL.Path)

	if len(actualSegments) != len(patternSegments) {
		return false
	}

	for i := range patternSegments {
		if isVariableSegment(patternSegments[i]) {
			continue
		}
		if patternSegments[i] != actualSegments[i] {
			return false
		}
	}

	return true
}

func isVariableSegment(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
}

func pathSegments(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return []string{}
	}

	parts := strings.Split(trimmed, "/")
	segments := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		segments = append(segments, p)
	}

	return segments
}

func GetUrlPath(urlInst string) (string, error) {
	parsed, err := url.Parse(urlInst)
	if err != nil {
		return "", errors.New("invalid URL")
	}

	return strings.Trim(parsed.Path, "/"), nil
}

func GetVariable(pattern string, urlInst string, varName string) (string, error) {
	if varName == "" {
		return "", errors.New("variable name must be provided")
	}

	normalizedPattern := strings.ReplaceAll(pattern, "\\", "/")
	normalizedURL := strings.ReplaceAll(urlInst, "\\", "/")

	patternURL, err := url.Parse(normalizedPattern)
	if err != nil {
		return "", errors.New("invalid pattern URL")
	}

	actualURL, err := url.Parse(normalizedURL)
	if err != nil {
		return "", errors.New("invalid instance URL")
	}

	patternSegments := pathSegments(patternURL.Path)
	actualSegments := pathSegments(actualURL.Path)

	if len(patternSegments) != len(actualSegments) {
		return "", errors.New("URL did not match expected pattern")
	}

	needle := "{" + varName + "}"
	for i, seg := range patternSegments {
		if !isVariableSegment(seg) {
			continue
		}
		if seg == needle {
			return actualSegments[i], nil
		}
	}

	return "", errors.New("pattern did not contain expected variable parameter")
}

func GetSlug(pattern string, urlInst string) (string, error) {
	return GetVariable(pattern, urlInst, "slug")
}
