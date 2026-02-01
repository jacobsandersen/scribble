package util

import (
	"errors"
	"strings"

	"github.com/gosimple/slug"
)

func GenerateSlug(doc Mf2Document) string {
	var nameText string
	var contentText string

	// Try to get text from "name" property
	if name, ok := doc.Properties["name"]; ok {
		nameText = extractTextFromProperty(name)
	}

	// Try to get text from "content" property
	if content, ok := doc.Properties["content"]; ok {
		contentText = extractTextFromProperty(content)
	}

	// Generate slug from name, limiting to 5 words
	var generatedSlug string
	if nameText != "" {
		nameWords := strings.Fields(nameText)
		if len(nameWords) > 5 {
			nameWords = nameWords[:5]
		}
		generatedSlug = slug.Make(strings.Join(nameWords, " "))
	}

	// If slug is less than 5 words and we have content, combine with content up to 5 words total
	if len(strings.Fields(nameText)) < 5 && contentText != "" {
		contentWords := strings.Fields(contentText)
		var combined []string
		if nameText != "" {
			combined = strings.Fields(nameText)
		}

		// Add words from content until we reach 5 words max
		for _, word := range contentWords {
			if len(combined) >= 5 {
				break
			}
			combined = append(combined, word)
		}

		if len(combined) > 0 {
			generatedSlug = slug.Make(strings.Join(combined, " "))
		}
	}

	// If still no slug, try content alone (limited to 5 words)
	if generatedSlug == "" && contentText != "" {
		contentWords := strings.Fields(contentText)
		if len(contentWords) > 5 {
			contentWords = contentWords[:5]
		}
		generatedSlug = slug.Make(strings.Join(contentWords, " "))
	}

	return generatedSlug
}

// SlugFromURL extracts the {slug} from a full URL based on the given pattern.
// Given a pattern like "domain.com/{year}/{month}/{slug}" and a URL like "domain.com/2024/06/my-post",
// this function extracts "my-post" as the slug. If the pattern does not contain {slug}, this is an error.
// If the url is not an instance of the pattern, this is an error.
func SlugFromURL(pattern string, url string) (string, error) {
	if !strings.Contains(pattern, "{slug}") {
		return "", errors.New("Pattern did not contain expected {slug} parameter")
	}
	partsPattern := strings.Split(pattern, "/")
	partsURL := strings.Split(url, "/")
	if len(partsPattern) != len(partsURL) {
		return "", errors.New("URL did not match expected pattern")
	}

	var slugValue string
	for i, partPattern := range partsPattern {
		if partPattern == "{slug}" {
			slugValue = partsURL[i]
			break
		}
	}

	if slugValue == "" {
		return "", errors.New("Could not find slug in URL based on pattern")
	}

	return slugValue, nil
}

// extractTextFromProperty extracts text from a property value ([]any)
func extractTextFromProperty(values []any) string {
	for _, val := range values {
		// Skip empty values
		if val == nil {
			continue
		}

		// Case 1: Direct string value
		if str, ok := val.(string); ok && str != "" {
			return str
		}

		// Case 2: Embedded object with HTML content
		if obj, ok := val.(map[string]any); ok {
			if htmlVals, hasHtml := obj["html"]; hasHtml {
				// Handle both {html: "..."} and {html: ["..."]}
				switch v := htmlVals.(type) {
				case string:
					if v != "" {
						// Extract text from HTML - 100 words is enough for slug generation
						return HtmlToText(v, 100)
					}
				case []any:
					if len(v) > 0 {
						if htmlStr, ok := v[0].(string); ok && htmlStr != "" {
							// Extract text from HTML - 100 words is enough for slug generation
							return HtmlToText(htmlStr, 100)
						}
					}
				}
			}
		}
	}

	return ""
}
