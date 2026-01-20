package content

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/util"
)

type d1Expectation struct {
	contains string
	rows     []map[string]any
	status   int
	success  bool
}

func newD1TestStore(t *testing.T, expectations []d1Expectation) *D1ContentStore {
	t.Helper()

	idx := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}

		if !strings.HasSuffix(r.URL.Path, "/query") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var req struct {
			SQL    string   `json:"sql"`
			Params []string `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		if idx >= len(expectations) {
			t.Fatalf("unexpected request for sql: %s", req.SQL)
		}

		exp := expectations[idx]
		idx++

		if !strings.Contains(req.SQL, exp.contains) {
			t.Fatalf("expected sql containing %q, got %q", exp.contains, req.SQL)
		}

		status := exp.status
		if status == 0 {
			status = http.StatusOK
		}

		w.WriteHeader(status)
		if !exp.success {
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "errors": []map[string]any{{"message": "fail"}}})
			return
		}

		result := map[string]any{"success": true}
		if exp.rows != nil {
			result["results"] = exp.rows
		}

		resp := map[string]any{
			"success": true,
			"result":  []map[string]any{result},
		}

		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.D1ContentStrategy{
		AccountID:  "acc",
		DatabaseID: "db",
		APIToken:   "token",
		PublicUrl:  "https://example.test",
		Endpoint:   srv.URL,
	}

	store, err := newD1ContentStoreWithClient(cfg, srv.Client())
	if err != nil {
		t.Fatalf("store init: %v", err)
	}

	return store
}

func TestD1ContentStore_CreateAndGet(t *testing.T) {
	doc := util.Mf2Document{Type: []string{"h-entry"}, Properties: map[string][]any{"slug": []any{"post-1"}}}

	payload, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal doc: %v", err)
	}

	store := newD1TestStore(t, []d1Expectation{
		{contains: "CREATE TABLE", success: true},
		{contains: "INSERT INTO", success: true},
		{contains: "SELECT doc", success: true, rows: []map[string]any{{"doc": string(payload)}}},
	})

	ctx := context.Background()
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

	fetched, err := store.Get(ctx, url)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if fetched.Properties["slug"][0] != "post-1" {
		t.Fatalf("unexpected fetched doc: %+v", fetched)
	}
}

func TestD1ContentStore_UpdateDeleteUndeleteExists(t *testing.T) {
	existing := util.Mf2Document{Type: []string{"h-entry"}, Properties: map[string][]any{"slug": []any{"entry-1"}, "category": []any{"old"}}}
	existingPayload, _ := json.Marshal(existing)

	updated := util.Mf2Document{Type: []string{"h-entry"}, Properties: map[string][]any{"slug": []any{"entry-1"}, "category": []any{"new"}}}
	updatedPayload, _ := json.Marshal(updated)

	deletedDoc := util.Mf2Document{Type: []string{"h-entry"}, Properties: map[string][]any{"slug": []any{"entry-1"}, "category": []any{"new"}, "deleted": []any{true}}}
	deletedPayload, _ := json.Marshal(deletedDoc)

	store := newD1TestStore(t, []d1Expectation{
		{contains: "CREATE TABLE", success: true},
		{contains: "SELECT doc", success: true, rows: []map[string]any{{"doc": string(existingPayload)}}},
		{contains: "UPDATE", success: true},
		{contains: "SELECT doc", success: true, rows: []map[string]any{{"doc": string(updatedPayload)}}},
		{contains: "UPDATE", success: true},
		{contains: "SELECT doc", success: true, rows: []map[string]any{{"doc": string(deletedPayload)}}},
		{contains: "UPDATE", success: true},
		{contains: "SELECT 1", success: true, rows: []map[string]any{{"1": 1}}},
	})

	ctx := context.Background()

	if _, err := store.Update(ctx, "https://example.test/entry-1", map[string][]any{"category": []any{"new"}}, nil, nil); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if err := store.Delete(ctx, "https://example.test/entry-1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if _, _, err := store.Undelete(ctx, "https://example.test/entry-1"); err != nil {
		t.Fatalf("undelete failed: %v", err)
	}

	exists, err := store.ExistsBySlug(ctx, "entry-1")
	if err != nil {
		t.Fatalf("exists failed: %v", err)
	}
	if !exists {
		t.Fatalf("expected slug to exist")
	}
}

func TestD1ContentStore_Get_NotFound(t *testing.T) {
	store := newD1TestStore(t, []d1Expectation{
		{contains: "CREATE TABLE", success: true},
		{contains: "SELECT doc", success: true, rows: []map[string]any{}},
	})

	if _, err := store.Get(context.Background(), "https://example.test/missing"); err == nil {
		t.Fatalf("expected ErrNotFound")
	} else if err != ErrNotFound {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestD1ContentStore_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "errors": []map[string]any{{"code": 100, "message": "bad"}}})
	}))
	t.Cleanup(srv.Close)

	cfg := &config.D1ContentStrategy{
		AccountID:  "acc",
		DatabaseID: "db",
		APIToken:   "token",
		PublicUrl:  "https://example.test",
		Endpoint:   srv.URL,
	}

	if _, err := newD1ContentStoreWithClient(cfg, srv.Client()); err == nil {
		t.Fatalf("expected schema failure due to api error")
	}
}

func TestD1ContentStore_UpdateSlugChange(t *testing.T) {
	existing := util.Mf2Document{
		Type:       []string{"h-entry"},
		Properties: map[string][]any{"slug": []any{"old-slug"}, "name": []any{"Old Title"}},
	}
	existingPayload, _ := json.Marshal(existing)

	store := newD1TestStore(t, []d1Expectation{
		{contains: "CREATE TABLE", success: true},
		// Test 1: Update with name change - should trigger slug change
		{contains: "SELECT doc", success: true, rows: []map[string]any{{"doc": string(existingPayload)}}}, // Get doc
		{contains: "SELECT 1", success: true, rows: []map[string]any{}},                                   // Check collision (no collision)
		{contains: "DELETE FROM", success: true},                                                          // Delete old slug
		{contains: "INSERT INTO", success: true},                                                          // Insert with new slug
		// Test 2: Direct slug replacement
		{contains: "SELECT doc", success: true, rows: []map[string]any{{"doc": string(existingPayload)}}}, // Get doc
		{contains: "SELECT 1", success: true, rows: []map[string]any{}},                                   // Check collision (no collision)
		{contains: "DELETE FROM", success: true},                                                          // Delete old slug
		{contains: "INSERT INTO", success: true},                                                          // Insert with custom slug
		// Test 3: Update without slug change
		{contains: "SELECT doc", success: true, rows: []map[string]any{{"doc": string(existingPayload)}}}, // Get doc
		{contains: "UPDATE", success: true}, // Simple update, no collision check needed
	})

	ctx := context.Background()

	// Test 1: Update name should change slug
	newURL, err := store.Update(ctx, "https://example.test/old-slug", map[string][]any{"name": []any{"New Awesome Title"}}, nil, nil)
	if err != nil {
		t.Fatalf("update with name change failed: %v", err)
	}
	if newURL != "https://example.test/new-awesome-title" {
		t.Fatalf("expected new URL https://example.test/new-awesome-title, got %s", newURL)
	}

	// Test 2: Direct slug replacement
	newURL2, err := store.Update(ctx, "https://example.test/old-slug", map[string][]any{"slug": []any{"custom-slug"}}, nil, nil)
	if err != nil {
		t.Fatalf("update with direct slug failed: %v", err)
	}
	if newURL2 != "https://example.test/custom-slug" {
		t.Fatalf("expected new URL https://example.test/custom-slug, got %s", newURL2)
	}

	// Test 3: Update without slug change
	newURL3, err := store.Update(ctx, "https://example.test/old-slug", map[string][]any{"category": []any{"test"}}, nil, nil)
	if err != nil {
		t.Fatalf("update without slug change failed: %v", err)
	}
	if newURL3 != "https://example.test/old-slug" {
		t.Fatalf("expected URL to remain old-slug, got %s", newURL3)
	}
}

func TestD1ContentStore_UpdateSlugCollision(t *testing.T) {
	existing := util.Mf2Document{
		Type:       []string{"h-entry"},
		Properties: map[string][]any{"slug": []any{"old-slug"}, "name": []any{"Old Title"}},
	}
	existingPayload, _ := json.Marshal(existing)

	store := newD1TestStore(t, []d1Expectation{
		{contains: "CREATE TABLE", success: true},
		// Update with name that would collide with existing slug
		{contains: "SELECT doc", success: true, rows: []map[string]any{{"doc": string(existingPayload)}}}, // Get doc
		{contains: "SELECT 1", success: true, rows: []map[string]any{{"1": 1}}},                           // Check collision - exists!
		{contains: "SELECT 1", success: true, rows: []map[string]any{}},                                   // Check UUID-suffixed slug - doesn't exist
		{contains: "DELETE FROM", success: true},                                                          // Delete old slug
		{contains: "INSERT INTO", success: true},                                                          // Insert with UUID-suffixed slug
	})

	ctx := context.Background()

	// Update name to something that would collide with an existing slug
	newURL, err := store.Update(ctx, "https://example.test/old-slug", map[string][]any{"name": []any{"Colliding Title"}}, nil, nil)
	if err != nil {
		t.Fatalf("update with collision failed: %v", err)
	}

	// Should have UUID appended due to collision
	if !strings.HasPrefix(newURL, "https://example.test/colliding-title-") {
		t.Fatalf("expected UUID suffix due to collision, got %s", newURL)
	}
}
