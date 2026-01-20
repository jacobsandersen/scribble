package content

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/indieinfra/scribble/server/util"
)

// deleteValues removes elements present in toRemove from values using deep equality.
func deleteValues(values []any, toRemove []any) []any {
	if len(values) == 0 || len(toRemove) == 0 {
		return values
	}

	var remaining []any
	for _, v := range values {
		if !containsValue(toRemove, v) {
			remaining = append(remaining, v)
		}
	}

	return remaining
}

func containsValue(list []any, value any) bool {
	for _, candidate := range list {
		if reflect.DeepEqual(candidate, value) {
			return true
		}
	}

	return false
}

func normalizeBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimRight(trimmed, "/")
	return trimmed + "/"
}

func extractSlug(doc util.Mf2Document) (string, error) {
	slugProp, ok := doc.Properties["slug"]
	if !ok || len(slugProp) == 0 {
		return "", fmt.Errorf("document must have a slug property")
	}

	slug, ok := slugProp[0].(string)
	if !ok || slug == "" {
		return "", fmt.Errorf("slug property must be a non-empty string")
	}

	return slug, nil
}

func applyMutations(doc *util.Mf2Document, replacements map[string][]any, additions map[string][]any, deletions any) {
	if doc.Properties == nil {
		doc.Properties = make(map[string][]any)
	}

	for key, values := range replacements {
		doc.Properties[key] = values
	}

	for key, values := range additions {
		doc.Properties[key] = append(doc.Properties[key], values...)
	}

	switch deletes := deletions.(type) {
	case map[string][]any:
		for key, valuesToRemove := range deletes {
			remaining := deleteValues(doc.Properties[key], valuesToRemove)
			if len(remaining) == 0 {
				delete(doc.Properties, key)
			} else {
				doc.Properties[key] = remaining
			}
		}
	case []string:
		for _, key := range deletes {
			delete(doc.Properties, key)
		}
	}
}

func deletedFlag(doc *util.Mf2Document) bool {
	if doc == nil || doc.Properties == nil {
		return false
	}

	values := doc.Properties["deleted"]
	if len(values) == 0 {
		return false
	}

	if b, ok := values[0].(bool); ok {
		return b
	}

	if s, ok := values[0].(string); ok {
		return strings.EqualFold(s, "true")
	}

	return false
}

// shouldRecomputeSlug checks if the mutations affect properties that should trigger slug recomputation.
// Returns true if "slug" is directly replaced, or if "name" or "content" are replaced/added.
func shouldRecomputeSlug(replacements map[string][]any, additions map[string][]any) bool {
	// Direct slug replacement always means we should use the new slug
	if _, hasSlug := replacements["slug"]; hasSlug {
		return true
	}

	// Check if name or content are being replaced or added
	if _, hasName := replacements["name"]; hasName {
		return true
	}
	if _, hasContent := replacements["content"]; hasContent {
		return true
	}
	if _, hasName := additions["name"]; hasName {
		return true
	}
	if _, hasContent := additions["content"]; hasContent {
		return true
	}

	return false
}

// computeNewSlug determines the new slug for a document after mutations.
// If the slug was explicitly set in replacements, use that.
// Otherwise, generate a new slug from name/content using util.GenerateSlug.
func computeNewSlug(doc *util.Mf2Document, replacements map[string][]any) (string, error) {
	// If slug was directly replaced, use it
	if slugVals, ok := replacements["slug"]; ok && len(slugVals) > 0 {
		if slug, ok := slugVals[0].(string); ok && slug != "" {
			return slug, nil
		}
		return "", fmt.Errorf("slug replacement must be a non-empty string")
	}

	// Generate slug from current document state (after mutations have been applied)
	generated := util.GenerateSlug(*doc)
	if generated == "" {
		return "", fmt.Errorf("failed to generate slug from document")
	}

	return generated, nil
}
