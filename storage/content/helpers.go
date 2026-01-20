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
	trimmed = strings.TrimSuffix(trimmed, "/")
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
