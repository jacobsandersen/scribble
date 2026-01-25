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

// StoreImpl implements Store using Cloudflare D1 via the HTTP API.
// It mirrors the schema of SQLContentStore to keep parity across backends.
type StoreImpl struct {
	cfg        *config.D1ContentStrategy
	pagination *config.Pagination
	client     *cloudflare.Client
	table      string
	publicURL  string
}

// NewD1ContentStore builds a StoreImpl and ensures the schema exists.
func NewD1ContentStore(cfg *config.Content) (*StoreImpl, error) {
	if cfg == nil {
		return nil, fmt.Errorf("d1 content config is nil")
	}

	d1Cfg := cfg.D1

	client := buildD1Client(d1Cfg)

	store := &StoreImpl{
		cfg:        d1Cfg,
		pagination: &cfg.Pagination,
		client:     client,
		table:      storageutil.DeriveTableName(d1Cfg.TablePrefix, "content"),
		publicURL:  storageutil.NormalizeBaseURL(cfg.PublicUrl),
	}

	if err := store.initSchema(context.Background()); err != nil {
		return nil, err
	}

	return store, nil
}

// buildD1Client creates a Cloudflare client configured with API token and optional custom endpoint.
// The httpClient parameter is used for testing; pass nil for production use.
func buildD1Client(cfg *config.D1ContentStrategy) *cloudflare.Client {
	opts := []option.RequestOption{option.WithAPIToken(strings.TrimSpace(cfg.APIToken))}

	if base := strings.TrimSpace(cfg.Endpoint); base != "" {
		opts = append(opts, option.WithBaseURL(strings.TrimSuffix(base, "/")))
	}

	return cloudflare.NewClient(opts...)
}

// initSchema ensures the content table exists in the D1 database.
// This also serves as a health check, validating connectivity and authentication.
func (cs *StoreImpl) initSchema(ctx context.Context) error {
	errMsg := "d1 initialization failed: %w"

	_, err := cs.executeQuery(ctx, cs.schemaQuery())
	if err != nil {
		return fmt.Errorf(errMsg, err)
	}

	return nil
}

// schemaQuery returns the CREATE TABLE statement for the content table.
func (cs *StoreImpl) schemaQuery() string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (doc TEXT PRIMARY KEY)`, cs.table)
}

// insertQuery builds the SQL for creating a new document.
func (cs *StoreImpl) insertQuery() string {
	return fmt.Sprintf("INSERT INTO %s (doc) VALUES (?)", cs.table)
}

// updateQuery builds the SQL for updating an existing document.
func (cs *StoreImpl) updateQuery() string {
	return fmt.Sprintf("UPDATE %s SET doc = ? WHERE json_extract(doc, '$.properties.slug') = json_array(?)", cs.table)
}

// selectQuery builds the SQL for retrieving a document by slug.
func (cs *StoreImpl) selectQuery() string {
	return fmt.Sprintf("SELECT doc FROM %s WHERE json_extract(doc, '$.properties.slug') = json_array(?) LIMIT 1", cs.table)
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

// selectMultipleQuery builds the SQL for retrieving multiple documents ordered by recency.
func (cs *StoreImpl) selectMultipleQuery(page int, limit int) string {
	page, limit, offset := cs.normalizePagination(page, limit)

	query := "SELECT doc FROM " + cs.table
	if cs.pagination.Enabled {
		query = fmt.Sprintf("%s LIMIT %d,%d", query, offset, cs.pagination.PerPage)
	}

	return query
}

func (cs *StoreImpl) selectCategoriesQuery(page int, limit int, withFilter bool) string {
	page, limit, offset := cs.normalizePagination(page, limit)

	query := fmt.Sprintf("SELECT DISTINCT c.value AS category FROM %s AS d JOIN json_each(d.doc, '$.properties.category') AS c", cs.table)
	if withFilter {
		query = fmt.Sprintf("%s WHERE c.value LIKE ? || '%%'", query)
	}

	if cs.pagination.Enabled {
		query = fmt.Sprintf("%s LIMIT %d OFFSET %d", query, limit, offset)
	}

	return query
}

// existsQuery builds the SQL for checking if a slug exists.
func (cs *StoreImpl) existsQuery() string {
	return fmt.Sprintf("SELECT 1 FROM %s WHERE json_extract(doc, '$.properties.slug') = json_array(?) LIMIT 1", cs.table)
}

func (cs *StoreImpl) Create(ctx context.Context, doc util.Mf2Document) (string, bool, error) {
	slug, err := content.ExtractSlug(doc)
	if err != nil {
		return "", false, err
	}

	url := cs.publicURL + slug

	payload, err := json.Marshal(doc)
	if err != nil {
		return "", false, err
	}

	if _, err := cs.executeQuery(ctx, cs.insertQuery(), string(payload)); err != nil {
		return "", false, err
	}

	return url, true, nil
}

func (cs *StoreImpl) Update(ctx context.Context, url string, replacements map[string][]any, additions map[string][]any, deletions any) (string, error) {
	oldSlug, err := util.SlugFromURL(url)
	if err != nil {
		return url, err
	}

	doc, err := cs.getDocBySlug(ctx, oldSlug)
	if err != nil {
		return url, err
	}

	content.ApplyMutations(doc, replacements, additions, deletions)

	// Check if slug needs to be recomputed
	var newSlug string
	if content.ShouldRecomputeSlug(replacements, additions) {
		proposedSlug, err := content.ComputeNewSlug(doc, replacements)
		if err != nil {
			return url, err
		}

		// Ensure the slug is unique; if collision, append UUID
		newSlug, err = content.EnsureUniqueSlug(ctx, cs, proposedSlug, oldSlug)
		if err != nil {
			return url, err
		}

		// Update the slug property in the document with the final unique slug
		doc.Properties["slug"] = []any{newSlug}
	} else {
		newSlug = oldSlug
	}

	payload, err := json.Marshal(doc)
	if err != nil {
		return url, err
	}

	newURL := cs.publicURL + newSlug

	// If slug changed, we need to insert the new row first, then delete the old one.
	// D1 doesn't support full transactions, so we simulate atomicity with manual rollback:
	// 1. INSERT the new row (collision already checked above)
	// 2. Verify the new row exists
	// 3. DELETE the old row
	// 4. If DELETE fails, DELETE the new row (rollback) to restore original state
	if newSlug != oldSlug {
		deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE json_extract(doc, '$.properties.slug') = json_array(?)", cs.table)

		// Step 1: Insert new row
		if _, err := cs.executeQuery(ctx, cs.insertQuery(), []any{newSlug, newURL, string(payload), content.HasDeletedFlag(doc)}); err != nil {
			return url, fmt.Errorf("failed to insert new row for slug change: %w", err)
		}

		// Step 2: Verify new row exists
		exists, err := cs.ExistsBySlug(ctx, newSlug)
		if err != nil {
			// Attempt rollback: delete the new row we just inserted
			_, _ = cs.executeQuery(ctx, deleteQuery, []any{newSlug})
			return url, fmt.Errorf("failed to verify new row after insert: %w", err)
		}
		if !exists {
			// Attempt rollback: delete the new row (though it wasn't found)
			_, _ = cs.executeQuery(ctx, deleteQuery, []any{newSlug})
			return url, fmt.Errorf("new row not found after insert, refusing to proceed")
		}

		// Step 3: Delete old row
		if _, err := cs.executeQuery(ctx, deleteQuery, []any{oldSlug}); err != nil {
			// ROLLBACK: Delete the new row to restore original state
			if _, rbErr := cs.executeQuery(ctx, deleteQuery, []any{newSlug}); rbErr != nil {
				return url, fmt.Errorf("failed to delete old row and rollback failed (system inconsistent): delete_error=%w, rollback_error=%v", err, rbErr)
			}
			return url, fmt.Errorf("failed to delete old row (rolled back successfully): %w", err)
		}
	} else {
		// No slug change, just update in place
		if _, err := cs.executeQuery(ctx, cs.updateQuery(), []any{string(payload), content.HasDeletedFlag(doc), oldSlug}); err != nil {
			return url, err
		}
	}

	return newURL, nil
}

func (cs *StoreImpl) Delete(ctx context.Context, url string) error {
	_, err := cs.setDeletedStatus(ctx, url, true)
	return err
}

func (cs *StoreImpl) Undelete(ctx context.Context, url string) (string, bool, error) {
	newURL, err := cs.setDeletedStatus(ctx, url, false)
	return newURL, false, err
}

func (cs *StoreImpl) Get(ctx context.Context, url string) (*util.Mf2Document, error) {
	slug, err := util.SlugFromURL(url)
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

// getDocBySlug retrieves and unmarshals a document from the database by its slug.
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

// setDeletedStatus updates the deleted flag on a document and persists it.
// It applies the change both to the document properties and the database column.
func (cs *StoreImpl) setDeletedStatus(ctx context.Context, url string, deleted bool) (string, error) {
	return cs.Update(ctx, url, map[string][]any{"deleted": {deleted}}, nil, nil)
}

func (cs *StoreImpl) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	rows, err := cs.executeQuery(ctx, cs.existsQuery(), slug)
	if err != nil {
		return false, err
	}

	return len(rows) > 0, nil
}

// executeQuery sends a SQL query to the D1 database and returns the result rows.
// Returns nil rows (no error) when the query succeeds but produces no results.
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

// convertParams converts query parameters to D1's string-based parameter format.
// Booleans are converted to "1" (true) or "0" (false); all other types use Sprint.
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
