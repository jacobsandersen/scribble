package content

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	cloudflare "github.com/cloudflare/cloudflare-go/v6"
	cfd1 "github.com/cloudflare/cloudflare-go/v6/d1"
	"github.com/cloudflare/cloudflare-go/v6/option"
	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/util"
)

// D1ContentStore implements ContentStore using Cloudflare D1 via the HTTP API.
// It mirrors the schema of SQLContentStore to keep parity across backends.
type D1ContentStore struct {
	cfg       *config.D1ContentStrategy
	client    *cloudflare.Client
	table     string
	publicURL string
}

// NewD1ContentStore builds a store and ensures the schema exists.
func NewD1ContentStore(cfg *config.D1ContentStrategy) (*D1ContentStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("d1 content config is nil")
	}

	table := deriveTableName(cfg.TablePrefix)
	client := buildD1Client(cfg, nil)

	store := &D1ContentStore{
		cfg:       cfg,
		client:    client,
		table:     table,
		publicURL: normalizeBaseURL(cfg.PublicUrl),
	}

	if err := store.initSchema(context.Background()); err != nil {
		return nil, err
	}

	return store, nil
}

// newD1ContentStoreWithClient creates a D1 store with a custom HTTP client.
// This is used for testing to inject a mock HTTP client.
func newD1ContentStoreWithClient(cfg *config.D1ContentStrategy, client *http.Client) (*D1ContentStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("d1 content config is nil")
	}

	table := deriveTableName(cfg.TablePrefix)
	cfClient := buildD1Client(cfg, client)

	store := &D1ContentStore{
		cfg:       cfg,
		client:    cfClient,
		table:     table,
		publicURL: normalizeBaseURL(cfg.PublicUrl),
	}

	if err := store.initSchema(context.Background()); err != nil {
		return nil, err
	}

	return store, nil
}

// deriveTableName constructs the content table name from the configured prefix.
// If no prefix is set, defaults to "scribble"; empty string produces "content".
func deriveTableName(prefix *string) string {
	p := "scribble"
	if prefix != nil {
		p = *prefix
	}

	if p == "" {
		return "content"
	}

	return p + "_content"
}

// buildD1Client creates a Cloudflare client configured with API token and optional custom endpoint.
// The httpClient parameter is used for testing; pass nil for production use.
func buildD1Client(cfg *config.D1ContentStrategy, httpClient *http.Client) *cloudflare.Client {
	opts := []option.RequestOption{option.WithAPIToken(strings.TrimSpace(cfg.APIToken))}

	if httpClient != nil {
		opts = append(opts, option.WithHTTPClient(httpClient))
	}

	if base := strings.TrimSpace(cfg.Endpoint); base != "" {
		opts = append(opts, option.WithBaseURL(strings.TrimSuffix(base, "/")))
	}

	return cloudflare.NewClient(opts...)
}

// initSchema ensures the content table exists in the D1 database.
// This also serves as a health check, validating connectivity and authentication.
func (cs *D1ContentStore) initSchema(ctx context.Context) error {
	_, err := cs.executeQuery(ctx, cs.schemaQuery(), nil)
	if err != nil {
		return fmt.Errorf("d1 initialization failed (check account_id, database_id, and api_token): %w", err)
	}
	return nil
}

// schemaQuery returns the CREATE TABLE statement for the content table.
func (cs *D1ContentStore) schemaQuery() string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
slug TEXT PRIMARY KEY,
url TEXT NOT NULL,
doc TEXT NOT NULL,
deleted BOOLEAN NOT NULL DEFAULT FALSE,
updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`, cs.table)
}

// insertQuery builds the SQL for creating a new document.
func (cs *D1ContentStore) insertQuery() string {
	return fmt.Sprintf("INSERT INTO %s (slug, url, doc, deleted, updated_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)", cs.table)
}

// updateQuery builds the SQL for updating an existing document.
func (cs *D1ContentStore) updateQuery() string {
	return fmt.Sprintf("UPDATE %s SET doc = ?, deleted = ?, updated_at = CURRENT_TIMESTAMP WHERE slug = ?", cs.table)
}

// selectQuery builds the SQL for retrieving a document by slug.
func (cs *D1ContentStore) selectQuery() string {
	return fmt.Sprintf("SELECT doc FROM %s WHERE slug = ? LIMIT 1", cs.table)
}

// existsQuery builds the SQL for checking if a slug exists.
func (cs *D1ContentStore) existsQuery() string {
	return fmt.Sprintf("SELECT 1 FROM %s WHERE slug = ? LIMIT 1", cs.table)
}

func (cs *D1ContentStore) Create(ctx context.Context, doc util.Mf2Document) (string, bool, error) {
	slug, err := extractSlug(doc)
	if err != nil {
		return "", false, err
	}

	url := cs.publicURL + slug

	payload, err := json.Marshal(doc)
	if err != nil {
		return "", false, err
	}

	if _, err := cs.executeQuery(ctx, cs.insertQuery(), []any{slug, url, string(payload), false}); err != nil {
		return "", false, err
	}

	return url, true, nil
}

func (cs *D1ContentStore) Update(ctx context.Context, url string, replacements map[string][]any, additions map[string][]any, deletions any) (string, error) {
	oldSlug, err := util.SlugFromURL(url)
	if err != nil {
		return url, err
	}

	doc, err := cs.getDocBySlug(ctx, oldSlug)
	if err != nil {
		return url, err
	}

	applyMutations(doc, replacements, additions, deletions)

	// Check if slug needs to be recomputed
	var newSlug string
	if shouldRecomputeSlug(replacements, additions) {
		newSlug, err = computeNewSlug(doc, replacements)
		if err != nil {
			return url, err
		}

		// Update the slug property in the document
		doc.Properties["slug"] = []any{newSlug}
	} else {
		newSlug = oldSlug
	}

	payload, err := json.Marshal(doc)
	if err != nil {
		return url, err
	}

	newURL := cs.publicURL + newSlug

	// If slug changed, we need to delete the old row and insert a new one
	if newSlug != oldSlug {
		deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE slug = ?", cs.table)
		if _, err := cs.executeQuery(ctx, deleteQuery, []any{oldSlug}); err != nil {
			return url, err
		}

		if _, err := cs.executeQuery(ctx, cs.insertQuery(), []any{newSlug, newURL, string(payload), deletedFlag(doc)}); err != nil {
			return url, err
		}
	} else {
		// No slug change, just update in place
		if _, err := cs.executeQuery(ctx, cs.updateQuery(), []any{string(payload), deletedFlag(doc), oldSlug}); err != nil {
			return url, err
		}
	}

	return newURL, nil
}

func (cs *D1ContentStore) Delete(ctx context.Context, url string) error {
	_, err := cs.setDeletedStatus(ctx, url, true)
	return err
}

func (cs *D1ContentStore) Undelete(ctx context.Context, url string) (string, bool, error) {
	newURL, err := cs.setDeletedStatus(ctx, url, false)
	return newURL, false, err
}

func (cs *D1ContentStore) Get(ctx context.Context, url string) (*util.Mf2Document, error) {
	slug, err := util.SlugFromURL(url)
	if err != nil {
		return nil, err
	}

	return cs.getDocBySlug(ctx, slug)
}

// getDocBySlug retrieves and unmarshals a document from the database by its slug.
func (cs *D1ContentStore) getDocBySlug(ctx context.Context, slug string) (*util.Mf2Document, error) {
	rows, err := cs.query(ctx, cs.selectQuery(), slug)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, ErrNotFound
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
func (cs *D1ContentStore) setDeletedStatus(ctx context.Context, url string, deleted bool) (string, error) {
	slug, err := util.SlugFromURL(url)
	if err != nil {
		return url, err
	}

	doc, err := cs.getDocBySlug(ctx, slug)
	if err != nil {
		return url, err
	}

	applyMutations(doc, map[string][]any{"deleted": []any{deleted}}, nil, nil)

	payload, err := json.Marshal(doc)
	if err != nil {
		return url, err
	}

	if _, err := cs.executeQuery(ctx, cs.updateQuery(), []any{string(payload), deleted, slug}); err != nil {
		return url, err
	}

	return cs.publicURL + slug, nil
}

func (cs *D1ContentStore) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	rows, err := cs.query(ctx, cs.existsQuery(), slug)
	if err != nil {
		return false, err
	}

	return len(rows) > 0, nil
}

func (cs *D1ContentStore) query(ctx context.Context, sql string, params ...any) ([]map[string]any, error) {
	return cs.executeQuery(ctx, sql, params)
}

// executeQuery sends a SQL query to the D1 database and returns the result rows.
// Returns nil rows (no error) when the query succeeds but produces no results.
func (cs *D1ContentStore) executeQuery(ctx context.Context, sql string, params []any) ([]map[string]any, error) {
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
