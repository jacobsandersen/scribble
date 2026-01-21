package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/util"
	storageutil "github.com/indieinfra/scribble/storage/util"
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
		publicURL:   storageutil.NormalizeBaseURL(cfg.PublicUrl),
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
		proposedSlug, err := computeNewSlug(doc, replacements)
		if err != nil {
			return url, err
		}

		// For SQL, we defer collision checking until inside the transaction
		// to prevent TOCTOU races. Just use the proposed slug for now.
		newSlug = proposedSlug

		// Update the slug property in the document with the proposed slug
		// (may be adjusted for uniqueness later inside the transaction)
		doc.Properties["slug"] = []any{newSlug}
	} else {
		newSlug = oldSlug
	}

	payload, err := json.Marshal(doc)
	if err != nil {
		return url, err
	}

	newURL := cs.publicURL + newSlug

	// Start a transaction for atomic update
	tx, err := cs.db.BeginTx(ctx, nil)
	if err != nil {
		return url, err
	}
	defer func() {
		// Rollback is safe to call after Commit; it will return sql.ErrTxDone
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			log.Printf("unexpected error during transaction rollback in Update: %v", rbErr)
		}
	}()

	// If slug changed, we need to delete the old row and insert a new one
	if newSlug != oldSlug {
		// Perform collision check inside transaction to prevent TOCTOU race
		finalSlug, err := cs.ensureUniqueSlugInTx(ctx, tx, newSlug, oldSlug)
		if err != nil {
			return url, err
		}

		// If collision detection changed the slug, update the document and URL
		if finalSlug != newSlug {
			newSlug = finalSlug
			newURL = cs.publicURL + newSlug
			doc.Properties["slug"] = []any{newSlug}

			// Re-marshal with updated slug
			payload, err = json.Marshal(doc)
			if err != nil {
				return url, err
			}
		}

		// Delete old entry
		deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE slug = %s", cs.table, cs.placeholderFor(1))
		if _, err := tx.ExecContext(ctx, deleteQuery, oldSlug); err != nil {
			return url, err
		}

		// Insert new entry with new slug
		if _, err := tx.ExecContext(ctx, cs.insertQuery(), newSlug, newURL, string(payload), deletedFlag(doc)); err != nil {
			return url, err
		}
	} else {
		// No slug change, just update in place
		if _, err := tx.ExecContext(ctx, cs.updateQuery(), string(payload), deletedFlag(doc), oldSlug); err != nil {
			return url, err
		}
	}

	if err := tx.Commit(); err != nil {
		return url, err
	}

	return newURL, nil
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

	applyMutations(doc, map[string][]any{"deleted": []any{deleted}}, nil, nil)

	payload, err := json.Marshal(doc)
	if err != nil {
		return url, err
	}

	// Use transaction for consistency with Update method
	tx, err := cs.db.BeginTx(ctx, nil)
	if err != nil {
		return url, err
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			log.Printf("unexpected error during transaction rollback in setDeletedStatus: %v", rbErr)
		}
	}()

	if _, err := tx.ExecContext(ctx, cs.updateQuery(), string(payload), deleted, slug); err != nil {
		return url, err
	}

	if err := tx.Commit(); err != nil {
		return url, err
	}

	return cs.publicURL + slug, nil
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

// ensureUniqueSlugInTx checks if the proposed slug already exists within a transaction.
// If it does, appends a UUID suffix to make it unique. Returns the final unique slug.
// This must be called within an active transaction to prevent TOCTOU races.
func (cs *SQLContentStore) ensureUniqueSlugInTx(ctx context.Context, tx *sql.Tx, proposedSlug, oldSlug string) (string, error) {
	// If the slug didn't actually change, no collision possible
	if proposedSlug == oldSlug {
		return proposedSlug, nil
	}

	// Check if the proposed slug already exists (within transaction)
	query := cs.existsQuery()
	row := tx.QueryRowContext(ctx, query, proposedSlug)

	var found int
	err := row.Scan(&found)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("failed to check slug existence: %w", err)
	}

	exists := (err == nil)

	// If it doesn't exist, we can use it as-is
	if !exists {
		return proposedSlug, nil
	}

	// Collision detected - append UUID to make it unique
	uniqueSlug := fmt.Sprintf("%s-%s", proposedSlug, uuid.New().String())

	// Sanity check: verify the UUID-suffixed slug doesn't exist either
	row = tx.QueryRowContext(ctx, query, uniqueSlug)
	err = row.Scan(&found)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("failed to check unique slug existence: %w", err)
	}

	if err == nil {
		// This should never happen in practice, but if it does, fail safely
		return "", fmt.Errorf("slug collision persists even after UUID suffix: %s", uniqueSlug)
	}

	return uniqueSlug, nil
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
