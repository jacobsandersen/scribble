package d1

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"slices"

	// Import the D1 driver - its init function registers the driver with database/sql
	_ "github.com/synehq/d1_go_sql"

	"github.com/jacobsandersen/scribble/config"
	"github.com/jacobsandersen/scribble/internal/db"
	"github.com/jacobsandersen/scribble/internal/db/schema"
	"github.com/jacobsandersen/scribble/server/util"
	"github.com/jacobsandersen/scribble/storage/content"
)

type StoreImpl struct {
	cfg        *config.D1ContentStrategy
	contentUrl string
	pagination *config.Pagination
	queries    *db.Queries
}

func NewD1ContentStore(cfg *config.Content) (*StoreImpl, error) {
	if cfg == nil {
		return nil, fmt.Errorf("d1 content config is nil")
	}

	d1cfg := cfg.D1

	backend, err := sql.Open("d1", fmt.Sprintf("d1://%s:%s@%s", d1cfg.AccountID, d1cfg.APIToken, d1cfg.DatabaseID))
	if err != nil {
		return nil, fmt.Errorf("failed to open d1 database connection: %w", err)
	}

	if _, err := backend.ExecContext(context.Background(), schema.Sqlite); err != nil {
		return nil, fmt.Errorf("failed to ensure d1 database schema: %w", err)
	}

	queries := db.New(backend)

	return &StoreImpl{
		cfg:        cfg.D1,
		contentUrl: cfg.ContentUrl,
		pagination: &cfg.Pagination,
		queries:    queries,
	}, nil
}

func (cs *StoreImpl) Create(ctx context.Context, doc util.Mf2Document) (bool, error) {
	payload, err := json.Marshal(doc)
	if err != nil {
		return false, err
	}

	if err := cs.queries.InsertDocument(ctx, string(payload)); err != nil {
		return false, err
	}

	return true, nil
}

func (cs *StoreImpl) Update(ctx context.Context, url string, replacements map[string][]any, additions map[string][]any, deletions any) (*util.Mf2Document, error) {
	oldSlug, err := util.SlugFromURL(cs.contentUrl, url)
	if err != nil {
		return nil, err
	}

	doc, err := cs.GetBySlug(ctx, oldSlug)
	if err != nil {
		return nil, err
	}

	content.ApplyMutations(doc, replacements, additions, deletions)

	newSlug, err := content.ExtractSlug(*doc)
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	if newSlug != oldSlug {
		if err := cs.queries.InsertDocument(ctx, string(payload)); err != nil {
			return nil, fmt.Errorf("failed to insert new row for slug change: %w", err)
		}

		if err := cs.queries.DeleteDocumentBySlug(ctx, content.StringToNullString(oldSlug)); err != nil {
			if rbErr := cs.queries.DeleteDocumentBySlug(ctx, content.StringToNullString(newSlug)); rbErr != nil {
				return nil, fmt.Errorf("failed to delete old row and rollback failed (system inconsistent): delete_error=%w, rollback_error=%v", err, rbErr)
			}

			return nil, fmt.Errorf("failed to delete old row after slug change: %w", err)
		}
	} else {
		if err := cs.queries.UpdateDocumentBySlug(ctx, db.UpdateDocumentBySlugParams{Doc: string(payload), Slug: content.StringToNullString(newSlug)}); err != nil {
			return nil, fmt.Errorf("failed to update document: %w", err)
		}
	}

	return doc, nil
}

func (cs *StoreImpl) Delete(ctx context.Context, url string) error {
	_, err := cs.Update(ctx, url, map[string][]any{"deleted": {true}}, nil, nil)
	return err
}

func (cs *StoreImpl) Undelete(ctx context.Context, url string) error {
	_, err := cs.Update(ctx, url, nil, nil, []string{"deleted"})
	return err
}

func (cs *StoreImpl) Get(ctx context.Context, url string) (*util.Mf2Document, error) {
	slug, err := util.SlugFromURL(cs.contentUrl, url)
	if err != nil {
		return nil, err
	}

	return cs.GetBySlug(ctx, slug)
}

func (cs *StoreImpl) GetBySlug(ctx context.Context, slug string) (*util.Mf2Document, error) {
	document, err := cs.queries.GetDocumentBySlug(ctx, content.StringToNullString(slug))
	if err != nil {
		return nil, err
	}

	doc, err := parseRowToDocument(document)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

func (cs *StoreImpl) List(ctx context.Context, page int, limit int) ([]*util.Mf2Document, error) {
	_, limit, offset := content.NormalizePagination(cs.pagination.PerPage, page, limit)

	rows, err := cs.queries.ListDocuments(ctx, db.ListDocumentsParams{Limit: int64(limit), Offset: int64(offset)})
	if err != nil {
		return nil, err
	}

	return parseRowsToDocuments(rows), nil
}

func (cs *StoreImpl) Query(ctx context.Context, page int, limit int, filter content.QueryDocumentsFilter) ([]*util.Mf2Document, error) {
	_, limit, offset := content.NormalizePagination(cs.pagination.PerPage, page, limit)

	params := content.QueryDocumentsParamsFromFilter(limit, offset, filter)

	rows, err := cs.queries.QueryDocuments(ctx, params)
	if err != nil {
		return nil, err
	}

	return parseRowsToDocuments(rows), nil
}

func (cs *StoreImpl) ListCategories(ctx context.Context, page int, limit int, filter string) ([]string, error) {
	var rows []string
	var err error

	_, limit, offset := content.NormalizePagination(cs.pagination.PerPage, page, limit)

	if filter != "" {
		rows, err = cs.queries.ListCategoriesLike(ctx, db.ListCategoriesLikeParams{Category: fmt.Sprintf("%%%s%%", filter), Limit: int64(limit), Offset: int64(offset)})
	} else {
		rows, err = cs.queries.ListCategories(ctx, db.ListCategoriesParams{Limit: int64(limit), Offset: int64(offset)})
	}

	if err != nil {
		return nil, err
	} else if len(rows) == 0 {
		return []string{}, nil
	}

	categories := make([]string, 0, len(rows))
	for _, cat := range rows {
		if !slices.Contains(categories, cat) {
			categories = append(categories, cat)
		}
	}

	return categories, nil
}

func (cs *StoreImpl) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	exists, err := cs.queries.DocExistsBySlug(ctx, content.StringToNullString(slug))
	if err != nil {
		return false, err
	}

	return exists != 0, nil
}

func parseRowsToDocuments(rows []string) []*util.Mf2Document {
	if len(rows) == 0 {
		return []*util.Mf2Document{}
	}

	docs := make([]*util.Mf2Document, 0, len(rows))
	for _, row := range rows {
		doc, err := parseRowToDocument(row)
		if err != nil {
			log.Println("warning: skipping document due to parse error:", err)
			continue
		}

		docs = append(docs, doc)
	}

	return docs
}

func parseRowToDocument(row string) (*util.Mf2Document, error) {
	if row == "" {
		return nil, fmt.Errorf("warning: tried to unmarshal empty document")
	}

	var doc util.Mf2Document
	if err := json.Unmarshal([]byte(row), &doc); err != nil {
		log.Println("warning: failed to unmarshal document json:", err)
		return nil, err
	}

	return &doc, nil
}
