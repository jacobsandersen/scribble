package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/util"
)

type placeholderStyle int

const (
	placeholderQuestion placeholderStyle = iota
	placeholderDollar
)

type SQLContentStore struct {
	cfg         *config.SQLContentStrategy
	db          *sql.DB
	table       string
	placeholder placeholderStyle
	publicURL   string
}

func NewSQLContentStore(cfg *config.SQLContentStrategy) (*SQLContentStore, error) {
	store, err := newSQLContentStoreWithDB(cfg, nil)
	if err != nil {
		return nil, err
	}

	driverName, err := resolveSQLDriverName(cfg.Driver)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(driverName, cfg.DSN)
	if err != nil {
		return nil, err
	}

	store.db = db

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := store.initSchema(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func newSQLContentStoreWithDB(cfg *config.SQLContentStrategy, db *sql.DB) (*SQLContentStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("content sql config is nil")
	}

	prefix := "scribble"
	if cfg.TablePrefix != nil {
		prefix = *cfg.TablePrefix
	}

	table := "content"
	if prefix != "" {
		table = prefix + "_content"
	}

	placeholder, err := detectPlaceholderStyle(cfg.Driver)
	if err != nil {
		return nil, err
	}

	return &SQLContentStore{
		cfg:         cfg,
		db:          db,
		table:       table,
		placeholder: placeholder,
		publicURL:   normalizeBaseURL(cfg.PublicUrl),
	}, nil
}

func detectPlaceholderStyle(driver string) (placeholderStyle, error) {
	driverName, err := resolveSQLDriverName(driver)
	if err != nil {
		return placeholderQuestion, err
	}

	if driverName == "pgx" {
		return placeholderDollar, nil
	}

	return placeholderQuestion, nil
}

func resolveSQLDriverName(driver string) (string, error) {
	switch strings.ToLower(driver) {
	case "postgres":
		return "pgx", nil
	case "mysql":
		return "mysql", nil
	default:
		return "", fmt.Errorf("unsupported sql driver %q", driver)
	}
}

func (cs *SQLContentStore) initSchema(ctx context.Context) error {
	_, err := cs.db.ExecContext(ctx, cs.schemaQuery())
	return err
}

func (cs *SQLContentStore) schemaQuery() string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
slug VARCHAR(255) PRIMARY KEY,
url TEXT NOT NULL,
doc TEXT NOT NULL,
deleted BOOLEAN NOT NULL DEFAULT FALSE,
updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`, cs.table)
}

func (cs *SQLContentStore) Create(ctx context.Context, doc util.Mf2Document) (string, bool, error) {
	slug, err := extractSlug(doc)
	if err != nil {
		return "", false, err
	}

	url := cs.publicURL + slug

	payload, err := json.Marshal(doc)
	if err != nil {
		return "", false, err
	}

	query := cs.insertQuery()
	if _, err := cs.db.ExecContext(ctx, query, slug, url, string(payload), false); err != nil {
		return "", false, err
	}

	return cs.publicURL + slug, true, nil
}

func (cs *SQLContentStore) Update(ctx context.Context, url string, replacements map[string][]any, additions map[string][]any, deletions any) (string, error) {
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

	_, err = cs.db.ExecContext(ctx, cs.updateQuery(), string(payload), deletedFlag(doc), slug)
	return cs.publicURL + slug, err
}

func (cs *SQLContentStore) Delete(ctx context.Context, url string) error {
	_, err := cs.setDeletedStatus(ctx, url, true)
	return err
}

func (cs *SQLContentStore) Undelete(ctx context.Context, url string) (string, bool, error) {
	newURL, err := cs.setDeletedStatus(ctx, url, false)
	return newURL, false, err
}

func (cs *SQLContentStore) Get(ctx context.Context, url string) (*util.Mf2Document, error) {
	slug, err := util.SlugFromURL(url)
	if err != nil {
		return nil, err
	}

	return cs.getDocBySlug(ctx, slug)
}

func (cs *SQLContentStore) getDocBySlug(ctx context.Context, slug string) (*util.Mf2Document, error) {
	query := cs.selectQuery()
	row := cs.db.QueryRowContext(ctx, query, slug)

	var raw string
	if err := row.Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	var doc util.Mf2Document
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, err
	}

	return &doc, nil
}

func (cs *SQLContentStore) setDeletedStatus(ctx context.Context, url string, deleted bool) (string, error) {
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

	_, err = cs.db.ExecContext(ctx, cs.updateQuery(), string(payload), deleted, slug)
	return cs.publicURL + slug, err
}

func (cs *SQLContentStore) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	query := cs.existsQuery()
	row := cs.db.QueryRowContext(ctx, query, slug)

	var found int
	if err := row.Scan(&found); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (cs *SQLContentStore) insertQuery() string {
	return fmt.Sprintf(
		"INSERT INTO %s (slug, url, doc, deleted, updated_at) VALUES (%s, %s, %s, %s, NOW())",
		cs.table,
		cs.placeholderFor(1),
		cs.placeholderFor(2),
		cs.placeholderFor(3),
		cs.placeholderFor(4),
	)
}

func (cs *SQLContentStore) updateQuery() string {
	return fmt.Sprintf(
		"UPDATE %s SET doc = %s, deleted = %s, updated_at = NOW() WHERE slug = %s",
		cs.table,
		cs.placeholderFor(1),
		cs.placeholderFor(2),
		cs.placeholderFor(3),
	)
}

func (cs *SQLContentStore) selectQuery() string {
	return fmt.Sprintf("SELECT doc FROM %s WHERE slug = %s", cs.table, cs.placeholderFor(1))
}

func (cs *SQLContentStore) existsQuery() string {
	return fmt.Sprintf("SELECT 1 FROM %s WHERE slug = %s", cs.table, cs.placeholderFor(1))
}

func (cs *SQLContentStore) placeholderFor(index int) string {
	if cs.placeholder == placeholderDollar {
		return fmt.Sprintf("$%d", index)
	}

	return "?"
}
