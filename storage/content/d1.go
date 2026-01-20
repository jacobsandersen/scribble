package content

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/util"
)

// D1ContentStore implements ContentStore using Cloudflare D1 via the HTTP API.
// It mirrors the schema of SQLContentStore to keep parity across backends.
type D1ContentStore struct {
	cfg       *config.D1ContentStrategy
	client    *http.Client
	endpoint  string
	table     string
	publicURL string
}

type d1Request struct {
	SQL    string `json:"sql"`
	Params []any  `json:"params"`
}

type d1APIError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type d1Result struct {
	Success bool             `json:"success"`
	Errors  []d1APIError     `json:"errors"`
	Results []map[string]any `json:"results"`
	Meta    map[string]any   `json:"meta"`
}

type d1Response struct {
	Success  bool         `json:"success"`
	Errors   []d1APIError `json:"errors"`
	Messages []string     `json:"messages"`
	Result   []d1Result   `json:"result"`
}

// NewD1ContentStore builds a store and ensures the schema exists.
func NewD1ContentStore(cfg *config.D1ContentStrategy) (*D1ContentStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("content d1 config is nil")
	}

	table := deriveTableName(cfg.TablePrefix)
	endpoint := buildD1Endpoint(cfg)

	store := &D1ContentStore{
		cfg:       cfg,
		client:    &http.Client{Timeout: 15 * time.Second},
		endpoint:  endpoint,
		table:     table,
		publicURL: strings.TrimSuffix(cfg.PublicUrl, "/"),
	}

	if err := store.initSchema(context.Background()); err != nil {
		return nil, err
	}

	return store, nil
}

// newD1ContentStoreWithClient is test-only to inject an http.Client.
func newD1ContentStoreWithClient(cfg *config.D1ContentStrategy, client *http.Client) (*D1ContentStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("content d1 config is nil")
	}

	table := deriveTableName(cfg.TablePrefix)
	endpoint := buildD1Endpoint(cfg)

	c := client
	if c == nil {
		c = &http.Client{Timeout: 15 * time.Second}
	}

	store := &D1ContentStore{
		cfg:       cfg,
		client:    c,
		endpoint:  endpoint,
		table:     table,
		publicURL: strings.TrimSuffix(cfg.PublicUrl, "/"),
	}

	if err := store.initSchema(context.Background()); err != nil {
		return nil, err
	}

	return store, nil
}

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

func buildD1Endpoint(cfg *config.D1ContentStrategy) string {
	base := strings.TrimSuffix(cfg.Endpoint, "/")
	if base == "" {
		base = "https://api.cloudflare.com/client/v4"
	}

	return fmt.Sprintf("%s/accounts/%s/d1/database/%s/raw", base, strings.Trim(cfg.AccountID, "/"), strings.Trim(cfg.DatabaseID, "/"))
}

func (cs *D1ContentStore) initSchema(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := cs.raw(ctx, cs.schemaQuery(), nil)
	return err
}

func (cs *D1ContentStore) schemaQuery() string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
slug TEXT PRIMARY KEY,
url TEXT NOT NULL,
doc TEXT NOT NULL,
deleted BOOLEAN NOT NULL DEFAULT FALSE,
updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`, cs.table)
}

func (cs *D1ContentStore) insertQuery() string {
	return fmt.Sprintf("INSERT INTO %s (slug, url, doc, deleted, updated_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)", cs.table)
}

func (cs *D1ContentStore) updateQuery() string {
	return fmt.Sprintf("UPDATE %s SET doc = ?, deleted = ?, updated_at = CURRENT_TIMESTAMP WHERE slug = ?", cs.table)
}

func (cs *D1ContentStore) selectQuery() string {
	return fmt.Sprintf("SELECT doc FROM %s WHERE slug = ? LIMIT 1", cs.table)
}

func (cs *D1ContentStore) existsQuery() string {
	return fmt.Sprintf("SELECT 1 FROM %s WHERE slug = ? LIMIT 1", cs.table)
}

func (cs *D1ContentStore) Create(ctx context.Context, doc util.Mf2Document) (string, bool, error) {
	slug, err := extractSlug(doc)
	if err != nil {
		return "", false, err
	}

	url := cs.publicURL + "/" + slug

	payload, err := json.Marshal(doc)
	if err != nil {
		return "", false, err
	}

	if _, err := cs.raw(ctx, cs.insertQuery(), []any{slug, url, string(payload), false}); err != nil {
		return "", false, err
	}

	return url, true, nil
}

func (cs *D1ContentStore) Update(ctx context.Context, url string, replacements map[string][]any, additions map[string][]any, deletions any) (string, error) {
	slug, err := util.SlugFromURL(url)
	if err != nil {
		return url, err
	}

	doc, err := cs.getDocBySlug(ctx, slug)
	if err != nil {
		return url, err
	}

	applyMutations(doc, replacements, additions, deletions)

	payload, err := json.Marshal(doc)
	if err != nil {
		return url, err
	}

	if _, err := cs.raw(ctx, cs.updateQuery(), []any{string(payload), deletedFlag(doc), slug}); err != nil {
		return url, err
	}

	return url, nil
}

func (cs *D1ContentStore) Delete(ctx context.Context, url string) error {
	_, err := cs.setDeletedStatus(ctx, url, true)
	return err
}

func (cs *D1ContentStore) Undelete(ctx context.Context, url string) (string, bool, error) {
	_, err := cs.setDeletedStatus(ctx, url, false)
	return url, false, err
}

func (cs *D1ContentStore) Get(ctx context.Context, url string) (*util.Mf2Document, error) {
	slug, err := util.SlugFromURL(url)
	if err != nil {
		return nil, err
	}

	return cs.getDocBySlug(ctx, slug)
}

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

func (cs *D1ContentStore) setDeletedStatus(ctx context.Context, url string, deleted bool) (string, error) {
	slug, err := util.SlugFromURL(url)
	if err != nil {
		return url, err
	}

	doc, err := cs.getDocBySlug(ctx, slug)
	if err != nil {
		return url, err
	}

	if doc.Properties == nil {
		doc.Properties = make(map[string][]any)
	}
	doc.Properties["deleted"] = []any{deleted}

	payload, err := json.Marshal(doc)
	if err != nil {
		return url, err
	}

	if _, err := cs.raw(ctx, cs.updateQuery(), []any{string(payload), deleted, slug}); err != nil {
		return url, err
	}

	return url, nil
}

func (cs *D1ContentStore) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	rows, err := cs.query(ctx, cs.existsQuery(), slug)
	if err != nil {
		return false, err
	}

	return len(rows) > 0, nil
}

func (cs *D1ContentStore) query(ctx context.Context, sql string, params ...any) ([]map[string]any, error) {
	res, err := cs.raw(ctx, sql, params)
	if err != nil {
		return nil, err
	}

	return res.Results, nil
}

func (cs *D1ContentStore) raw(ctx context.Context, sql string, params []any) (*d1Result, error) {
	if params == nil {
		params = []any{}
	}

	body, err := json.Marshal(d1Request{SQL: sql, Params: params})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cs.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cs.cfg.APIToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := cs.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("d1 request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var envelope d1Response
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("failed to decode d1 response: %w", err)
	}

	if !envelope.Success {
		return nil, fmt.Errorf("d1 api error: %s", joinD1Errors(envelope.Errors))
	}

	if len(envelope.Result) == 0 {
		return &d1Result{Success: true}, nil
	}

	res := envelope.Result[0]
	if !res.Success {
		return nil, fmt.Errorf("d1 statement error: %s", joinD1Errors(res.Errors))
	}

	return &res, nil
}

func joinD1Errors(errors []d1APIError) string {
	if len(errors) == 0 {
		return "unknown error"
	}

	parts := make([]string, 0, len(errors))
	for _, e := range errors {
		if e.Code != 0 {
			parts = append(parts, fmt.Sprintf("%d:%s", e.Code, e.Message))
			continue
		}
		parts = append(parts, e.Message)
	}

	return strings.Join(parts, "; ")
}
