package content

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/util"
)

func TestSQLContentStore_CreateAndGet_PostgresPlaceholders(t *testing.T) {
	store, mock := newSQLTestStore(t, "postgres", nil)
	ctx := context.Background()

	doc := util.Mf2Document{Type: []string{"h-entry"}, Properties: map[string][]any{"slug": []any{"post-1"}}}

	mock.ExpectExec(regexp.QuoteMeta(store.insertQuery())).
		WithArgs("post-1", "https://example.test/post-1", sqlmock.AnyArg(), false).
		WillReturnResult(sqlmock.NewResult(1, 1))

	url, now, err := store.Create(ctx, doc)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if !now {
		t.Fatalf("expected immediate availability")
	}

	if url != "https://example.test/post-1" {
		t.Fatalf("unexpected url: %s", url)
	}

	payload, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal doc: %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(store.selectQuery())).
		WithArgs("post-1").
		WillReturnRows(sqlmock.NewRows([]string{"doc"}).AddRow(string(payload)))

	fetched, err := store.Get(ctx, url)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if fetched == nil || fetched.Properties["slug"][0] != "post-1" {
		t.Fatalf("unexpected fetched doc: %+v", fetched)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSQLContentStore_UpdateDeleteUndelete_MySQLPlaceholders(t *testing.T) {
	store, mock := newSQLTestStore(t, "mysql", nil)
	ctx := context.Background()

	existing := util.Mf2Document{Type: []string{"h-entry"}, Properties: map[string][]any{"slug": []any{"entry-1"}, "name": []any{"old"}}}
	existingPayload, err := json.Marshal(existing)
	if err != nil {
		t.Fatalf("marshal existing: %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(store.selectQuery())).
		WithArgs("entry-1").
		WillReturnRows(sqlmock.NewRows([]string{"doc"}).AddRow(string(existingPayload)))

	mock.ExpectExec(regexp.QuoteMeta(store.updateQuery())).
		WithArgs(jsonContains("\"name\":[\"new\"]"), false, "entry-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	_, err = store.Update(ctx, "https://example.test/entry-1", map[string][]any{"name": []any{"new"}}, nil, nil)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	updated := util.Mf2Document{Type: []string{"h-entry"}, Properties: map[string][]any{"slug": []any{"entry-1"}, "name": []any{"new"}}}
	updatedPayload, err := json.Marshal(updated)
	if err != nil {
		t.Fatalf("marshal updated: %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(store.selectQuery())).
		WithArgs("entry-1").
		WillReturnRows(sqlmock.NewRows([]string{"doc"}).AddRow(string(updatedPayload)))

	mock.ExpectExec(regexp.QuoteMeta(store.updateQuery())).
		WithArgs(jsonContains("\"deleted\":[true]"), true, "entry-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := store.Delete(ctx, "https://example.test/entry-1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	deletedDoc := util.Mf2Document{Type: []string{"h-entry"}, Properties: map[string][]any{"slug": []any{"entry-1"}, "name": []any{"new"}, "deleted": []any{true}}}
	deletedPayload, err := json.Marshal(deletedDoc)
	if err != nil {
		t.Fatalf("marshal deleted: %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(store.selectQuery())).
		WithArgs("entry-1").
		WillReturnRows(sqlmock.NewRows([]string{"doc"}).AddRow(string(deletedPayload)))

	mock.ExpectExec(regexp.QuoteMeta(store.updateQuery())).
		WithArgs(jsonContains("\"deleted\":[false]"), false, "entry-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if _, _, err := store.Undelete(ctx, "https://example.test/entry-1"); err != nil {
		t.Fatalf("undelete failed: %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta(store.existsQuery())).
		WithArgs("entry-1").
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	exists, err := store.ExistsBySlug(ctx, "entry-1")
	if err != nil {
		t.Fatalf("exists check failed: %v", err)
	}

	if !exists {
		t.Fatalf("expected slug to exist")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSQLContentStore_ExistsBySlug_NoRows(t *testing.T) {
	store, mock := newSQLTestStore(t, "postgres", nil)
	ctx := context.Background()

	mock.ExpectQuery(regexp.QuoteMeta(store.existsQuery())).
		WithArgs("missing").
		WillReturnRows(sqlmock.NewRows([]string{"1"}))

	exists, err := store.ExistsBySlug(ctx, "missing")
	if err != nil {
		t.Fatalf("exists failed: %v", err)
	}
	if exists {
		t.Fatalf("expected missing slug to be false")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestSQLContentStore_GetDocBySlug_NotFound(t *testing.T) {
	store, mock := newSQLTestStore(t, "postgres", nil)
	ctx := context.Background()

	mock.ExpectQuery(regexp.QuoteMeta(store.selectQuery())).
		WithArgs("missing").
		WillReturnRows(sqlmock.NewRows([]string{"doc"}))

	if _, err := store.getDocBySlug(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestNewSQLContentStore_InvalidDriver(t *testing.T) {
	cfg := &config.SQLContentStrategy{Driver: "invalid", DSN: "ignored"}
	if _, err := NewSQLContentStore(cfg); err == nil {
		t.Fatalf("expected error for invalid driver")
	}
}

func TestNewSQLContentStore_DefaultTablePrefix(t *testing.T) {
	cfg := &config.SQLContentStrategy{Driver: "postgres", DSN: "ignored", PublicUrl: "https://example.test"}
	store, err := newSQLContentStoreWithDB(cfg, nil)
	if err != nil {
		t.Fatalf("store setup failed: %v", err)
	}

	if store.table != "scribble_content" {
		t.Fatalf("expected default table name scribble_content, got %s", store.table)
	}
}

func TestNewSQLContentStore_CustomTablePrefix(t *testing.T) {
	shared := "shared"
	cfg := &config.SQLContentStrategy{Driver: "postgres", DSN: "ignored", PublicUrl: "https://example.test", TablePrefix: &shared}
	store, err := newSQLContentStoreWithDB(cfg, nil)
	if err != nil {
		t.Fatalf("store setup failed: %v", err)
	}

	if store.table != "shared_content" {
		t.Fatalf("expected table to use prefix: %s", store.table)
	}
}

func TestNewSQLContentStore_EmptyTablePrefix(t *testing.T) {
	empty := ""
	cfg := &config.SQLContentStrategy{Driver: "postgres", DSN: "ignored", PublicUrl: "https://example.test", TablePrefix: &empty}
	store, err := newSQLContentStoreWithDB(cfg, nil)
	if err != nil {
		t.Fatalf("store setup failed: %v", err)
	}

	if store.table != "content" {
		t.Fatalf("expected empty prefix to yield content, got %s", store.table)
	}
}

func TestNewSQLContentStore_InitSchemaFailure(t *testing.T) {
	cfg := &config.SQLContentStrategy{Driver: "mysql", DSN: "user:pass@tcp(127.0.0.1:0)/db", PublicUrl: "https://example.test"}

	store, err := NewSQLContentStore(cfg)
	if err == nil {
		_ = store.db.Close()
		t.Fatalf("expected schema/init to fail for unreachable database")
	}

	var opErr *net.OpError
	if !errors.As(err, &opErr) && !errors.Is(err, sql.ErrConnDone) {
		t.Fatalf("unexpected error type: %v", err)
	}
}

func newSQLTestStore(t *testing.T, driver string, prefix *string) (*SQLContentStore, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}

	cfg := &config.SQLContentStrategy{Driver: driver, DSN: "ignored", PublicUrl: "https://example.test", TablePrefix: prefix}
	store, err := newSQLContentStoreWithDB(cfg, db)
	if err != nil {
		t.Fatalf("store setup: %v", err)
	}

	mock.ExpectExec(regexp.QuoteMeta(store.schemaQuery())).WillReturnResult(sqlmock.NewResult(0, 0))
	if err := store.initSchema(context.Background()); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	return store, mock
}

type jsonContains string

func (m jsonContains) Match(v driver.Value) bool {
	s, ok := v.(string)
	return ok && strings.Contains(s, string(m))
}
