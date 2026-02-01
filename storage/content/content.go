package content

import (
	"context"

	"github.com/indieinfra/scribble/server/util"
)

type Store interface {
	Create(ctx context.Context, doc util.Mf2Document) (immediate bool, err error)

	Update(ctx context.Context, url string, replacements map[string][]any, additions map[string][]any, deletions any) (doc *util.Mf2Document, err error)

	Delete(ctx context.Context, url string) (err error)

	Undelete(ctx context.Context, url string) (err error)

	Get(ctx context.Context, url string) (doc *util.Mf2Document, err error)

	List(ctx context.Context, page int, limit int) (results []util.Mf2Document, err error)

	ListCategories(ctx context.Context, page int, limit int, filter string) (results []string, err error)

	ExistsBySlug(ctx context.Context, slug string) (exists bool, err error)
}
