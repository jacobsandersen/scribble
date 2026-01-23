package content

import (
	"context"
	"fmt"
	"reflect"
	"strings"

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

	for key, values := range replacements {
		doc.Properties[key] = values
	}

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

func HasDeletedFlag(doc *util.Mf2Document) bool {
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

// ShouldRecomputeSlug checks if the mutations affect properties that should trigger slug recomputation.
// Returns true if "slug" is directly replaced with a non-empty value,
// or if "name" or "content" are replaced/added with non-empty values.
func ShouldRecomputeSlug(replacements map[string][]any, additions map[string][]any) bool {
	// Direct slug replacement - but only if non-empty
	if slugVals, hasSlug := replacements["slug"]; hasSlug && len(slugVals) > 0 {
		return true
	}

	// Check if name or content are being replaced with non-empty values
	if nameVals, hasName := replacements["name"]; hasName && len(nameVals) > 0 {
		return true
	}
	if contentVals, hasContent := replacements["content"]; hasContent && len(contentVals) > 0 {
		return true
	}

	// Check if name or content are being added with non-empty values
	if nameVals, hasName := additions["name"]; hasName && len(nameVals) > 0 {
		return true
	}
	if contentVals, hasContent := additions["content"]; hasContent && len(contentVals) > 0 {
		return true
	}

	return false
}

// ComputeNewSlug determines the new slug for a document after mutations.
// If the slug was explicitly set in replacements, use that.
// Otherwise, generate a new slug from name/content using util.GenerateSlug.
func ComputeNewSlug(doc *util.Mf2Document, replacements map[string][]any) (string, error) {
	// If slug was directly replaced, validate it
	if slugVals, ok := replacements["slug"]; ok {
		if len(slugVals) == 0 {
			return "", fmt.Errorf("slug replacement cannot be empty array")
		}
		if slug, ok := slugVals[0].(string); ok && slug != "" {
			return slug, nil
		}
		return "", fmt.Errorf("slug replacement must be a non-empty string")
	}

	// Generate slug from current document state (after mutations have been applied)
	generated := util.GenerateSlug(*doc)
	if generated == "" {
		return "", fmt.Errorf("cannot generate slug: document has no name or content properties with text to derive slug from")
	}

	return generated, nil
}

// EnsureUniqueSlug checks if the proposed slug already exists (excluding the old slug).
// If it does, appends a UUID suffix to make it unique. Returns the final unique slug.
func EnsureUniqueSlug(ctx context.Context, store Store, proposedSlug, oldSlug string) (string, error) {
	// If the slug didn't actually change, no collision possible
	if proposedSlug == oldSlug {
		return proposedSlug, nil
	}

	// Check if the proposed slug already exists
	exists, err := store.ExistsBySlug(ctx, proposedSlug)
	if err != nil {
		return "", fmt.Errorf("failed to check slug existence: %w", err)
	}

	// If it doesn't exist, we can use it as-is
	if !exists {
		return proposedSlug, nil
	}

	// Collision detected - append UUID to make it unique
	uniqueSlug := fmt.Sprintf("%s-%s", proposedSlug, uuid.New().String())

	// Sanity check: verify the UUID-suffixed slug doesn't exist either
	// (extremely unlikely but theoretically possible)
	exists, err = store.ExistsBySlug(ctx, uniqueSlug)
	if err != nil {
		return "", fmt.Errorf("failed to check unique slug existence: %w", err)
	}

	if exists {
		// This should never happen in practice, but if it does, fail safely
		return "", fmt.Errorf("slug collision persists even after UUID suffix: %s", uniqueSlug)
	}

	return uniqueSlug, nil
}
