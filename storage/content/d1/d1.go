package d1

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/cloudflare/cloudflare-go/v6"
	cfd1 "github.com/cloudflare/cloudflare-go/v6/d1"
	"github.com/cloudflare/cloudflare-go/v6/option"
	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/util"
	"github.com/indieinfra/scribble/storage/content"
	storageutil "github.com/indieinfra/scribble/storage/util"
)

type StoreImpl struct {
	cfg           *config.D1ContentStrategy
	contentUrl    string
	pagination    *config.Pagination
	client        *cloudflare.Client
	contentTable  string
	categoryTable string
}

func NewD1ContentStore(cfg *config.Content) (*StoreImpl, error) {
	if cfg == nil {
		return nil, fmt.Errorf("d1 content config is nil")
	}

	store := &StoreImpl{
		cfg:           cfg.D1,
		contentUrl:    cfg.ContentUrl,
		pagination:    &cfg.Pagination,
		client:        buildD1Client(cfg.D1),
		contentTable:  storageutil.DeriveTableName(cfg.D1.TablePrefix, "content"),
		categoryTable: storageutil.DeriveTableName(cfg.D1.TablePrefix, "categories"),
	}

	if err := store.initSchema(context.Background()); err != nil {
		return nil, err
	}

	return store, nil
}

func buildD1Client(cfg *config.D1ContentStrategy) *cloudflare.Client {
	opts := []option.RequestOption{option.WithAPIToken(strings.TrimSpace(cfg.APIToken))}

	if base := strings.TrimSpace(cfg.Endpoint); base != "" {
		opts = append(opts, option.WithBaseURL(strings.TrimSuffix(base, "/")))
	}

	return cloudflare.NewClient(opts...)
}

func (cs *StoreImpl) initSchema(ctx context.Context) error {
	errMsg := "d1 initialization failed: %w"

	for _, query := range cs.initQueries() {
		if _, err := cs.executeQuery(ctx, query); err != nil {
			return fmt.Errorf(errMsg, err)
		}
	}

	return nil
}

func (cs *StoreImpl) initQueries() []string {
	return []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
						id INTEGER PRIMARY KEY, 
						doc TEXT NOT NULL
					)`, cs.contentTable),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_doc_slug ON %s(json_extract(doc, '$.properties.slug'))`, cs.contentTable),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_doc_created ON %s(json_extract(doc, '$.properties.created_at'))`, cs.contentTable),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
						doc_id INTEGER NOT NULL, 
						category TEXT NOT NULL,
						PRIMARY KEY (doc_id, category),
						FOREIGN KEY (doc_id) REFERENCES %s(id) ON DELETE CASCADE
					) 
					`, cs.categoryTable, cs.contentTable),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_category_value ON %s(category)`, cs.categoryTable),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_category_cover ON %s(category, doc_id)`, cs.categoryTable),
		fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS trg_add_categories
						AFTER INSERT ON %s
						BEGIN
							INSERT INTO %s (doc_id, category) SELECT NEW.id, json_each.value FROM json_each(NEW.doc, '$.properties.category');
						END`, cs.contentTable, cs.categoryTable),
		fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS trg_update_categories
						AFTER UPDATE OF doc ON %s
						WHEN json_extract(OLD.doc, '$.properties.category') IS NOT json_extract(NEW.doc, '$.properties.category')
						BEGIN
							DELETE FROM %s WHERE doc_id = OLD.id;
							INSERT INTO %s (doc_id, category) SELECT NEW.id, json_each.value FROM json_each(NEW.doc, '$.properties.category');
						END`, cs.contentTable, cs.categoryTable, cs.categoryTable),
	}
}

func (cs *StoreImpl) insertQuery() string {
	return fmt.Sprintf("INSERT INTO %s (doc) VALUES (?)", cs.contentTable)
}

func (cs *StoreImpl) updateQuery() string {
	return fmt.Sprintf("UPDATE %s SET doc = ? WHERE json_extract(doc, '$.properties.slug') = json_array(?)", cs.contentTable)
}

func (cs *StoreImpl) selectQuery() string {
	return fmt.Sprintf("SELECT doc FROM %s WHERE json_extract(doc, '$.properties.slug') = json_array(?) LIMIT 1", cs.contentTable)
}

func (cs *StoreImpl) normalizePagination(page int, limit int) (int, int, int) {
	if page < 1 {
		page = 1
	}

	if limit <= 0 || limit > cs.pagination.PerPage {
		limit = cs.pagination.PerPage
	}

	offset := 0
	if page > 1 {
		offset = (page - 1) * limit
	}

	return page, limit, offset
}

func (cs *StoreImpl) selectMultipleQuery(page int, limit int) string {
	page, limit, offset := cs.normalizePagination(page, limit)

	query := "SELECT doc FROM " + cs.contentTable + " ORDER BY json_extract(doc, '$.properties.created_at') DESC"
	if cs.pagination.Enabled {
		query = fmt.Sprintf("%s LIMIT %d OFFSET %d", query, limit, offset)
	}

	return query
}

func (cs *StoreImpl) selectCategoriesQuery(page int, limit int, withFilter bool) string {
	page, limit, offset := cs.normalizePagination(page, limit)

	query := fmt.Sprintf("SELECT DISTINCT category FROM %s", cs.categoryTable)

	if withFilter {
		query = fmt.Sprintf("%s WHERE category LIKE ? || '%%'", query)
	}

	if cs.pagination.Enabled {
		query = fmt.Sprintf("%s LIMIT %d OFFSET %d", query, limit, offset)
	}

	return query
}

func (cs *StoreImpl) existsQuery() string {
	return fmt.Sprintf("SELECT 1 FROM %s WHERE json_extract(doc, '$.properties.slug') = json_array(?) LIMIT 1", cs.contentTable)
}

func (cs *StoreImpl) Create(ctx context.Context, doc util.Mf2Document) (bool, error) {
	payload, err := json.Marshal(doc)
	if err != nil {
		return false, err
	}

	if _, err := cs.executeQuery(ctx, cs.insertQuery(), string(payload)); err != nil {
		return false, err
	}

	return true, nil
}

func (cs *StoreImpl) Update(ctx context.Context, url string, replacements map[string][]any, additions map[string][]any, deletions any) (*util.Mf2Document, error) {
	oldSlug, err := util.SlugFromURL(cs.contentUrl, url)
	if err != nil {
		return nil, err
	}

	doc, err := cs.getDocBySlug(ctx, oldSlug)
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
		if _, err := cs.executeQuery(ctx, cs.insertQuery(), string(payload)); err != nil {
			return nil, fmt.Errorf("failed to insert new row for slug change: %w", err)
		}

		deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE json_extract(doc, '$.properties.slug') = json_array(?)", cs.contentTable)
		if _, err := cs.executeQuery(ctx, deleteQuery, oldSlug); err != nil {
			if _, rbErr := cs.executeQuery(ctx, deleteQuery, newSlug); rbErr != nil {
				return nil, fmt.Errorf("failed to delete old row and rollback failed (system inconsistent): delete_error=%w, rollback_error=%v", err, rbErr)
			}
			return nil, fmt.Errorf("failed to delete old row (rolled back successfully): %w", err)
		}
	} else {
		if _, err := cs.executeQuery(ctx, cs.updateQuery(), string(payload), newSlug); err != nil {
			return nil, err
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

	return cs.getDocBySlug(ctx, slug)
}

func (cs *StoreImpl) List(ctx context.Context, page int, limit int) ([]util.Mf2Document, error) {
	rows, err := cs.executeQuery(ctx, cs.selectMultipleQuery(page, limit))
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return []util.Mf2Document{}, nil
	}

	docs := make([]util.Mf2Document, 0, len(rows))
	for _, row := range rows {
		raw, ok := row["doc"].(string)
		if !ok || raw == "" {
			log.Println("warning: no document found in row")
			continue
		}

		var doc util.Mf2Document
		if err := json.Unmarshal([]byte(raw), &doc); err != nil {
			log.Println("warning: failed to unmarshal document json:", err)
			continue
		}

		docs = append(docs, doc)
	}

	return docs, nil
}

func (cs *StoreImpl) ListCategories(ctx context.Context, page int, limit int, filter string) ([]string, error) {
	var rows []map[string]any
	var err error

	if filter != "" {
		rows, err = cs.executeQuery(ctx, cs.selectCategoriesQuery(page, limit, true), filter)
	} else {
		rows, err = cs.executeQuery(ctx, cs.selectCategoriesQuery(page, limit, false))
	}

	if err != nil {
		return nil, err
	} else if len(rows) == 0 {
		return []string{}, nil
	}

	categories := make([]string, 0, len(rows))
	for _, row := range rows {
		cat, ok := row["category"].(string)
		if !ok {
			continue
		}

		if !slices.Contains(categories, cat) {
			categories = append(categories, cat)
		}
	}

	return categories, nil
}

func (cs *StoreImpl) getDocBySlug(ctx context.Context, slug string) (*util.Mf2Document, error) {
	rows, err := cs.executeQuery(ctx, cs.selectQuery(), slug)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, content.ErrNotFound
	}

	raw, ok := rows[0]["doc"].(string)
	if !ok || raw == "" {
		return nil, fmt.Errorf("doc column missing or not a string")
	}

	var doc util.Mf2Document
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, err
	}

	return &doc, nil
}

func (cs *StoreImpl) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	rows, err := cs.executeQuery(ctx, cs.existsQuery(), slug)
	if err != nil {
		return false, err
	}

	return len(rows) > 0, nil
}

func (cs *StoreImpl) executeQuery(ctx context.Context, sql string, params ...any) ([]map[string]any, error) {
	body := cfd1.DatabaseQueryParamsBodyD1SingleQuery{Sql: cloudflare.F(sql)}
	if len(params) > 0 {
		body.Params = cloudflare.F(convertParams(params))
	}

	resp, err := cs.client.D1.Database.Query(ctx, cs.cfg.DatabaseID, cfd1.DatabaseQueryParams{
		AccountID: cloudflare.F(strings.TrimSpace(cs.cfg.AccountID)),
		Body:      body,
	})
	if err != nil {
		return nil, err
	}

	if resp == nil || len(resp.Result) == 0 {
		return nil, nil
	}

	result := resp.Result[0]
	if !result.Success {
		return nil, fmt.Errorf("d1 query execution failed")
	}

	rows := make([]map[string]any, 0, len(result.Results))
	for _, r := range result.Results {
		m, ok := r.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unexpected row type %T", r)
		}
		rows = append(rows, m)
	}

	return rows, nil
}

func convertParams(params []any) []string {
	if len(params) == 0 {
		return nil
	}

	out := make([]string, 0, len(params))
	for _, p := range params {
		switch v := p.(type) {
		case bool:
			if v {
				out = append(out, "1")
			} else {
				out = append(out, "0")
			}
		default:
			out = append(out, fmt.Sprint(p))
		}
	}

	return out
}
