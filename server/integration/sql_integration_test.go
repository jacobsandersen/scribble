//go:build testcontainers
// +build testcontainers

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/handler/get"
	"github.com/indieinfra/scribble/server/handler/post"
	"github.com/indieinfra/scribble/server/state"
	"github.com/indieinfra/scribble/server/util"
	"github.com/indieinfra/scribble/storage/content"
	"github.com/indieinfra/scribble/storage/media"
)

func stringPtr(s string) *string {
	return &s
}

func newPostgresState(t *testing.T) *state.ScribbleState {
	t.Helper()

	ctx := context.Background()

	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Cleanup(func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate postgres container: %v", err)
		}
	})

	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	cfg := &config.Config{
		Debug:  false,
		Server: config.Server{Limits: config.ServerLimits{MaxPayloadSize: 1 << 20, MaxFileSize: 1 << 20, MaxMultipartMem: 1 << 20}},
		Micropub: config.Micropub{
			MeUrl:         "https://example.test/me",
			TokenEndpoint: "https://example.test/token",
		},
		Content: config.Content{
			Strategy: "sql",
			SQL: &config.SQLContentStrategy{
				Driver:      "postgres",
				DSN:         connStr,
				TablePrefix: stringPtr("test"),
				PublicUrl:   "https://example.test/content/",
			},
		},
		Media: config.Media{Strategy: "noop"},
	}

	store, err := content.NewSQLContentStore(cfg.Content.SQL)
	if err != nil {
		t.Fatalf("failed to create postgres content store: %v", err)
	}

	return &state.ScribbleState{
		Cfg:          cfg,
		ContentStore: store,
		MediaStore:   &media.NoopMediaStore{},
	}
}

func newMySQLState(t *testing.T) *state.ScribbleState {
	t.Helper()

	ctx := context.Background()

	mysqlContainer, err := mysql.Run(ctx,
		"mysql:8.0",
		mysql.WithDatabase("testdb"),
		mysql.WithUsername("testuser"),
		mysql.WithPassword("testpass"),
	)
	if err != nil {
		t.Fatalf("failed to start mysql container: %v", err)
	}

	t.Cleanup(func() {
		if err := mysqlContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate mysql container: %v", err)
		}
	})

	connStr, err := mysqlContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	cfg := &config.Config{
		Debug:  false,
		Server: config.Server{Limits: config.ServerLimits{MaxPayloadSize: 1 << 20, MaxFileSize: 1 << 20, MaxMultipartMem: 1 << 20}},
		Micropub: config.Micropub{
			MeUrl:         "https://example.test/me",
			TokenEndpoint: "https://example.test/token",
		},
		Content: config.Content{
			Strategy: "sql",
			SQL: &config.SQLContentStrategy{
				Driver:      "mysql",
				DSN:         connStr,
				TablePrefix: stringPtr("test"),
				PublicUrl:   "https://example.test/content/",
			},
		},
		Media: config.Media{Strategy: "noop"},
	}

	store, err := content.NewSQLContentStore(cfg.Content.SQL)
	if err != nil {
		t.Fatalf("failed to create mysql content store: %v", err)
	}

	return &state.ScribbleState{
		Cfg:          cfg,
		ContentStore: store,
		MediaStore:   &media.NoopMediaStore{},
	}
}

func TestPostgres_BasicCreateAndRetrieve(t *testing.T) {
	st := newPostgresState(t)

	body := `{"type":["h-entry"],"properties":{"name":["Postgres Test"],"content":["Content"]}}`
	req := httptest.NewRequest("POST", "/micropub", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("expected location header")
	}

	// Retrieve
	req = httptest.NewRequest("GET", "/micropub?q=source&url="+location, nil)
	rec = httptest.NewRecorder()
	withToken(st.Cfg, get.DispatchGet(st)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var retrieved util.Mf2Document
	json.NewDecoder(rec.Body).Decode(&retrieved)

	if len(retrieved.Properties["name"]) == 0 || retrieved.Properties["name"][0].(string) != "Postgres Test" {
		t.Errorf("unexpected name: %v", retrieved.Properties["name"])
	}
}

func TestPostgres_SlugCollision(t *testing.T) {
	st := newPostgresState(t)

	body := `{"type":["h-entry"],"properties":{"name":["Collision"],"content":["Test"]}}`

	// Create first post
	req := httptest.NewRequest("POST", "/micropub", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)

	location1 := rec.Header().Get("Location")

	// Create second post with same name
	req = httptest.NewRequest("POST", "/micropub", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)

	location2 := rec.Header().Get("Location")

	if location1 == location2 {
		t.Error("expected different URLs for colliding slugs")
	}
}

func TestPostgres_Update(t *testing.T) {
	st := newPostgresState(t)

	createBody := `{"type":["h-entry"],"properties":{"name":["Original"],"content":["Content"]}}`
	req := httptest.NewRequest("POST", "/micropub", bytes.NewReader([]byte(createBody)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)

	location := rec.Header().Get("Location")

	// Update content (doesn't change slug)
	updateBody := `{"action":"update","url":"` + location + `","replace":{"content":["Updated content"]}}`
	req = httptest.NewRequest("POST", "/micropub", bytes.NewReader([]byte(updateBody)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent && rec.Code != http.StatusCreated {
		t.Fatalf("expected 204 or 201, got %d", rec.Code)
	}

	if rec.Code == http.StatusCreated {
		if loc := rec.Header().Get("Location"); loc != "" {
			location = loc
		}
	}

	// Verify (follow potential new location)
	req = httptest.NewRequest("GET", "/micropub?q=source&url="+location, nil)
	rec = httptest.NewRecorder()
	withToken(st.Cfg, get.DispatchGet(st)).ServeHTTP(rec, req)

	var updated util.Mf2Document
	json.NewDecoder(rec.Body).Decode(&updated)

	if len(updated.Properties["content"]) == 0 || updated.Properties["content"][0].(string) != "Updated content" {
		t.Errorf("content not updated: %v", updated.Properties["content"])
	}
}

func TestPostgres_Delete(t *testing.T) {
	st := newPostgresState(t)

	body := `{"type":["h-entry"],"properties":{"name":["Delete Test"],"content":["Content"]}}`
	req := httptest.NewRequest("POST", "/micropub", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)

	location := rec.Header().Get("Location")

	// Delete
	deleteBody := `{"action":"delete","url":"` + location + `"}`
	req = httptest.NewRequest("POST", "/micropub", bytes.NewReader([]byte(deleteBody)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	// Verify deleted flag
	req = httptest.NewRequest("GET", "/micropub?q=source&url="+location, nil)
	rec = httptest.NewRecorder()
	withToken(st.Cfg, get.DispatchGet(st)).ServeHTTP(rec, req)

	var deleted util.Mf2Document
	json.NewDecoder(rec.Body).Decode(&deleted)

	if len(deleted.Properties["deleted"]) == 0 || deleted.Properties["deleted"][0].(bool) != true {
		t.Error("expected deleted flag to be true")
	}
}

func TestMySQL_BasicCreateAndRetrieve(t *testing.T) {
	st := newMySQLState(t)

	body := `{"type":["h-entry"],"properties":{"name":["MySQL Test"],"content":["Content"]}}`
	req := httptest.NewRequest("POST", "/micropub", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("expected location header")
	}

	// Retrieve
	req = httptest.NewRequest("GET", "/micropub?q=source&url="+location, nil)
	rec = httptest.NewRecorder()
	withToken(st.Cfg, get.DispatchGet(st)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var retrieved util.Mf2Document
	json.NewDecoder(rec.Body).Decode(&retrieved)

	if len(retrieved.Properties["name"]) == 0 || retrieved.Properties["name"][0].(string) != "MySQL Test" {
		t.Errorf("unexpected name: %v", retrieved.Properties["name"])
	}
}

func TestMySQL_SlugCollision(t *testing.T) {
	st := newMySQLState(t)

	body := `{"type":["h-entry"],"properties":{"name":["Collision"],"content":["Test"]}}`

	// Create first post
	req := httptest.NewRequest("POST", "/micropub", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)

	location1 := rec.Header().Get("Location")

	// Create second post with same name
	req = httptest.NewRequest("POST", "/micropub", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)

	location2 := rec.Header().Get("Location")

	if location1 == location2 {
		t.Error("expected different URLs for colliding slugs")
	}
}
