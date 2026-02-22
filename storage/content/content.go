package content

import (
	"context"

	"github.com/jacobsandersen/scribble/server/util"
)

type QueryDocumentsFilter struct {
	Type             *string
	Slug             *string
	Category         *string
	CreatedYear      *int64
	CreatedMonth     *int64
	CreatedDay       *int64
	CreatedWeekday   *int64
	CreatedWeek      *int64
	CreatedDayOfYear *int64
}

type Store interface {
	Create(ctx context.Context, doc util.Mf2Document) (immediate bool, err error)

	Update(ctx context.Context, url string, replacements map[string][]any, additions map[string][]any, deletions any) (doc *util.Mf2Document, err error)

	Delete(ctx context.Context, url string) (err error)

	Undelete(ctx context.Context, url string) (err error)

	Get(ctx context.Context, url string) (doc *util.Mf2Document, err error)

	GetBySlug(ctx context.Context, slug string) (doc *util.Mf2Document, err error)

	List(ctx context.Context, page int, limit int) (results []*util.Mf2Document, err error)

	Query(ctx context.Context, page int, limit int, filter QueryDocumentsFilter) (results []*util.Mf2Document, err error)

	ListCategories(ctx context.Context, page int, limit int, filter string) (results []string, err error)

	ExistsBySlug(ctx context.Context, slug string) (exists bool, err error)
}
