package content

import (
	"context"
	"database/sql"
	"fmt"
	"maps"
	"reflect"

	"github.com/google/uuid"
	"github.com/indieinfra/scribble/internal/db"
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

func StringToNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

func NormalizePagination(perPage, page, limit int) (int, int, int) {
	if page < 1 {
		page = 1
	}

	if limit <= 0 || limit > perPage {
		limit = perPage
	}

	offset := 0
	if page > 1 {
		offset = (page - 1) * limit
	}

	return page, limit, offset
}

func QueryDocumentsParamsFromFilter(limit, offset int, filter QueryDocumentsFilter) db.QueryDocumentsParams {
	var (
		slug             sql.NullString
		category         string
		createdYear      sql.NullInt64
		createdMonth     sql.NullInt64
		createdDay       sql.NullInt64
		createdWeekday   sql.NullInt64
		createdWeek      sql.NullInt64
		createdDayOfYear sql.NullInt64

		applySlug             int64
		applyCategory         int64
		applyCreatedYear      int64
		applyCreatedMonth     int64
		applyCreatedDay       int64
		applyCreatedWeekday   int64
		applyCreatedWeek      int64
		applyCreatedDayOfYear int64
	)

	if filter.Slug != nil {
		applySlug = 1
		slug = sql.NullString{String: *filter.Slug, Valid: true}
	}
	if filter.Category != nil {
		applyCategory = 1
		category = *filter.Category
	}
	if filter.CreatedYear != nil {
		applyCreatedYear = 1
		createdYear = sql.NullInt64{Int64: *filter.CreatedYear, Valid: true}
	}
	if filter.CreatedMonth != nil {
		applyCreatedMonth = 1
		createdMonth = sql.NullInt64{Int64: *filter.CreatedMonth, Valid: true}
	}
	if filter.CreatedDay != nil {
		applyCreatedDay = 1
		createdDay = sql.NullInt64{Int64: *filter.CreatedDay, Valid: true}
	}
	if filter.CreatedWeekday != nil {
		applyCreatedWeekday = 1
		createdWeekday = sql.NullInt64{Int64: *filter.CreatedWeekday, Valid: true}
	}
	if filter.CreatedWeek != nil {
		applyCreatedWeek = 1
		createdWeek = sql.NullInt64{Int64: *filter.CreatedWeek, Valid: true}
	}
	if filter.CreatedDayOfYear != nil {
		applyCreatedDayOfYear = 1
		createdDayOfYear = sql.NullInt64{Int64: *filter.CreatedDayOfYear, Valid: true}
	}

	return db.QueryDocumentsParams{
		Column1:          applySlug,
		Slug:             slug,
		Column3:          applyCategory,
		Category:         category,
		Column5:          applyCreatedYear,
		CreatedYear:      createdYear,
		Column7:          applyCreatedMonth,
		CreatedMonth:     createdMonth,
		Column9:          applyCreatedDay,
		CreatedDay:       createdDay,
		Column11:         applyCreatedWeekday,
		CreatedWeekday:   createdWeekday,
		Column13:         applyCreatedWeek,
		CreatedWeek:      createdWeek,
		Column15:         applyCreatedDayOfYear,
		CreatedDayOfYear: createdDayOfYear,
		Offset:           int64(offset),
		Limit:            int64(limit),
	}
}
