package content

import (
	"context"
	"fmt"
	"maps"
	"reflect"

	"github.com/google/uuid"
	"github.com/indieinfra/scribble/server/util"
)

// DeleteValues removes elements present in toRemove from values using deep equality.
func DeleteValues(values []any, toRemove []any) []any {
	if len(values) == 0 || len(toRemove) == 0 {
		return values
	}

	var remaining []any
	for _, v := range values {
		if !ContainsValue(toRemove, v) {
			remaining = append(remaining, v)
		}
	}

	return remaining
}

func ContainsValue(list []any, value any) bool {
	for _, candidate := range list {
		if reflect.DeepEqual(candidate, value) {
			return true
		}
	}

	return false
}

func ExtractSlug(doc util.Mf2Document) (string, error) {
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

func ApplyMutations(doc *util.Mf2Document, replacements map[string][]any, additions map[string][]any, deletions any) {
	if doc.Properties == nil {
		doc.Properties = make(map[string][]any)
	}

	if replacements == nil {
		replacements = map[string][]any{}
	}

	replacements["updated_at"] = []any{util.CurrentLocalTimeRFC3339()}

	maps.Copy(doc.Properties, replacements)

	for key, values := range additions {
		doc.Properties[key] = append(doc.Properties[key], values...)
	}

	switch deletes := deletions.(type) {
	case map[string][]any:
		for key, valuesToRemove := range deletes {
			remaining := DeleteValues(doc.Properties[key], valuesToRemove)
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

func EnsureUniqueSlug(ctx context.Context, store Store, slug string) (string, error) {
	exists, err := store.ExistsBySlug(ctx, slug)
	if err != nil {
		return "", fmt.Errorf("failed to check slug existence: %w", err)
	}

	if !exists {
		return slug, nil
	}

	return fmt.Sprintf("%s-%s", slug, uuid.New().String()), nil
}
