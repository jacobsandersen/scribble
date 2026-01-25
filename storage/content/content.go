package content

import (
	"context"

	"github.com/indieinfra/scribble/server/util"
)

type Store interface {
	// Create accepts a Micropub document and stores it, returning the URL where the
	// object can be located. If the creation fails, it will return a non-nil error.
	Create(ctx context.Context, doc util.Mf2Document) (string, bool, error)

	// Update accepts an ID that refers to an existing document, and change sets to apply.
	// If the update fails, an error will be returned. Otherwise, nil is returned.
	Update(ctx context.Context, url string, replacements map[string][]any, additions map[string][]any, deletions any) (string, error)

	// Delete accepts an ID that refers to an existing document. If the object existed, it will
	// be marked deleted (deleted=true). It is up to the user to stop displaying a deleted object.
	Delete(ctx context.Context, url string) error

	// Undelete accepts an ID that refers to an existing document. If the object existed, it will
	// be marked undeleted (deleted=false).
	Undelete(ctx context.Context, url string) (string, bool, error)

	// Get accepts an ID and returns the matching mf2 document, if any. If no object is found, a non-nil
	// error will be returned and the document pointer will be nil.
	Get(ctx context.Context, url string) (*util.Mf2Document, error)

	// List returns a collection of documents respecting the provided page.
	// If pagination is disabled, the page parameter is ignored and all documents are returned.
	List(ctx context.Context, page int, limit int) ([]util.Mf2Document, error)

	// ListCategories returns a collection of categories previously seen by Scribble.
	// This specifically looks for documents containing a "category" property.
	ListCategories(ctx context.Context, page int, limit int, filter string) ([]string, error)

	// ExistsBySlug accepts a slug and returns whether a post exists by that slug. If an error occurs while
	// traversing the git tree, a non-nil error will be returned
	ExistsBySlug(ctx context.Context, slug string) (bool, error)
}
