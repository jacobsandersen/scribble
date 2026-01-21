package content

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/util"
)

func setupFilesystemTest(t *testing.T) (*FilesystemContentStore, string, func()) {
	t.Helper()

	tmpDir := t.TempDir()

	cfg := &config.FilesystemContentStrategy{
		Path:      tmpDir,
		PublicUrl: "https://example.com/",
	}

	store, err := NewFilesystemContentStore(cfg)
	if err != nil {
		t.Fatalf("NewFilesystemContentStore: %v", err)
	}

	cleanup := func() {
		// t.TempDir handles cleanup
	}

	return store, tmpDir, cleanup
}

func TestNewFilesystemContentStore(t *testing.T) {
	t.Run("creates directory if missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		nestedPath := filepath.Join(tmpDir, "nested", "path")

		cfg := &config.FilesystemContentStrategy{
			Path:      nestedPath,
			PublicUrl: "https://example.com/",
		}

		store, err := NewFilesystemContentStore(cfg)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
			t.Fatal("expected directory to be created")
		}

		if store.basePath != nestedPath {
			t.Errorf("basePath = %q, want %q", store.basePath, nestedPath)
		}
	})

	t.Run("rebuilds index from existing files", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a document file manually
		doc := util.Mf2Document{
			Type: []string{"h-entry"},
			Properties: map[string][]any{
				"slug":    []any{"test-post"},
				"content": []any{"test content"},
			},
		}

		docJSON, _ := json.MarshalIndent(doc, "", "  ")
		docPath := filepath.Join(tmpDir, "test-post.json")
		if err := os.WriteFile(docPath, docJSON, 0644); err != nil {
			t.Fatal(err)
		}

		cfg := &config.FilesystemContentStrategy{
			Path:      tmpDir,
			PublicUrl: "https://example.com/",
		}

		store, err := NewFilesystemContentStore(cfg)
		if err != nil {
			t.Fatalf("NewFilesystemContentStore: %v", err)
		}

		exists, err := store.ExistsBySlug(context.Background(), "test-post")
		if err != nil {
			t.Fatalf("ExistsBySlug: %v", err)
		}
		if !exists {
			t.Error("expected slug to exist in index")
		}
	})

	t.Run("nil config returns error", func(t *testing.T) {
		_, err := NewFilesystemContentStore(nil)
		if err == nil {
			t.Error("expected error for nil config")
		}
	})
}

func TestFilesystemContentStore_Create(t *testing.T) {
	t.Run("creates document with default pattern", func(t *testing.T) {
		store, tmpDir, cleanup := setupFilesystemTest(t)
		defer cleanup()

		doc := util.Mf2Document{
			Type: []string{"h-entry"},
			Properties: map[string][]any{
				"slug":    []any{"my-first-post"},
				"content": []any{"Hello World"},
			},
		}

		url, created, err := store.Create(context.Background(), doc)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		if !created {
			t.Error("expected created=true")
		}

		if url != "https://example.com/my-first-post" {
			t.Errorf("url = %q, want %q", url, "https://example.com/my-first-post")
		}

		// Verify file exists
		filePath := filepath.Join(tmpDir, "my-first-post.json")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Error("expected file to exist")
		}

		// Verify content
		data, _ := os.ReadFile(filePath)
		var saved util.Mf2Document
		if err := json.Unmarshal(data, &saved); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		slugProp, ok := saved.Properties["slug"]
		if !ok {
			t.Fatal("missing slug property")
		}
		if len(slugProp) == 0 || slugProp[0] != "my-first-post" {
			t.Error("slug mismatch")
		}
	})

	t.Run("detects duplicate slug", func(t *testing.T) {
		store, _, cleanup := setupFilesystemTest(t)
		defer cleanup()

		doc := util.Mf2Document{
			Type: []string{"h-entry"},
			Properties: map[string][]any{
				"slug":    []any{"duplicate"},
				"content": []any{"First"},
			},
		}

		_, _, err := store.Create(context.Background(), doc)
		if err != nil {
			t.Fatalf("Create 1: %v", err)
		}

		// Try to create again with same slug
		doc.Properties["content"] = []any{"Second"}
		url, _, err := store.Create(context.Background(), doc)
		if err != nil {
			t.Fatalf("Create 2: %v", err)
		}

		// Should have generated a new slug with UUID suffix
		slug, _ := util.SlugFromURL(url)
		if slug == "duplicate" {
			t.Error("expected slug to be modified for duplicate")
		}
	})

	t.Run("handles custom path pattern", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := &config.FilesystemContentStrategy{
			Path:        tmpDir,
			PublicUrl:   "https://example.com/",
			PathPattern: "{year}/{month}/{slug}.json",
		}

		store, err := NewFilesystemContentStore(cfg)
		if err != nil {
			t.Fatalf("NewFilesystemContentStore: %v", err)
		}

		doc := util.Mf2Document{
			Type: []string{"h-entry"},
			Properties: map[string][]any{
				"slug":    []any{"test"},
				"content": []any{"Test"},
			},
		}

		_, _, err = store.Create(context.Background(), doc)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		// File should be in year/month subdirectory
		matches, _ := filepath.Glob(filepath.Join(tmpDir, "*", "*", "test.json"))
		if len(matches) != 1 {
			t.Errorf("expected 1 file matching pattern, got %d", len(matches))
		}
	})
}

func TestFilesystemContentStore_Update(t *testing.T) {
	t.Run("updates content in place when slug doesn't change", func(t *testing.T) {
		store, _, cleanup := setupFilesystemTest(t)
		defer cleanup()

		doc := util.Mf2Document{
			Type: []string{"h-entry"},
			Properties: map[string][]any{
				"slug":    []any{"keep-this-slug"},
				"content": []any{"Original"},
			},
		}

		url, _, _ := store.Create(context.Background(), doc)

		// Update just a custom property, not name/content/slug
		replacements := map[string][]any{
			"category": {"test"},
		}

		newURL, err := store.Update(context.Background(), url, replacements, nil, nil)
		if err != nil {
			t.Fatalf("Update: %v", err)
		}

		if newURL != url {
			t.Errorf("URL should not change when slug is not recomputed: got %q, want %q", newURL, url)
		}

		// Verify category was updated
		fetched, _ := store.Get(context.Background(), url)
		categoryProp := fetched.Properties["category"]
		if len(categoryProp) == 0 {
			t.Fatal("missing category property")
		}
		category := categoryProp[0].(string)
		if category != "test" {
			t.Errorf("category = %q, want %q", category, "test")
		}
	})

	t.Run("renames file when slug changes", func(t *testing.T) {
		store, tmpDir, cleanup := setupFilesystemTest(t)
		defer cleanup()

		doc := util.Mf2Document{
			Type: []string{"h-entry"},
			Properties: map[string][]any{
				"slug":    []any{"old-slug"},
				"name":    []any{"Original Title"},
				"content": []any{"Content"},
			},
		}

		url, _, _ := store.Create(context.Background(), doc)

		// Update name, which should trigger slug recomputation
		replacements := map[string][]any{
			"name": {"New Title"},
		}

		newURL, err := store.Update(context.Background(), url, replacements, nil, nil)
		if err != nil {
			t.Fatalf("Update: %v", err)
		}

		if newURL == url {
			t.Error("URL should change when slug is recomputed")
		}

		// Old file should not exist
		oldPath := filepath.Join(tmpDir, "old-slug.json")
		if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
			t.Error("old file should be removed")
		}

		// New file should exist
		newSlug, _ := util.SlugFromURL(newURL)
		newPath := filepath.Join(tmpDir, newSlug+".json")
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			t.Error("new file should exist")
		}
	})

	t.Run("detects missing document", func(t *testing.T) {
		store, _, cleanup := setupFilesystemTest(t)
		defer cleanup()

		_, err := store.Update(
			context.Background(),
			"https://example.com/missing",
			map[string][]any{"content": {"Updated"}},
			nil,
			nil,
		)

		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestFilesystemContentStore_Delete(t *testing.T) {
	t.Run("marks document as deleted", func(t *testing.T) {
		store, _, cleanup := setupFilesystemTest(t)
		defer cleanup()

		doc := util.Mf2Document{
			Type: []string{"h-entry"},
			Properties: map[string][]any{
				"slug":    []any{"delete-me"},
				"content": []any{"Content"},
			},
		}

		url, _, _ := store.Create(context.Background(), doc)

		if err := store.Delete(context.Background(), url); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		// Document should still exist but have deleted property
		fetched, err := store.Get(context.Background(), url)
		if err != nil {
			t.Fatalf("Get after Delete: %v", err)
		}

		deleted, ok := fetched.Properties["deleted"]
		if !ok {
			t.Fatal("expected deleted property")
		}

		if len(deleted) == 0 {
			t.Error("deleted property should have at least one element")
		} else if deleted[0] != true {
			t.Error("deleted property should be [true]")
		}
	})
}

func TestFilesystemContentStore_Undelete(t *testing.T) {
	t.Run("removes deleted property", func(t *testing.T) {
		store, _, cleanup := setupFilesystemTest(t)
		defer cleanup()

		doc := util.Mf2Document{
			Type: []string{"h-entry"},
			Properties: map[string][]any{
				"slug":    []any{"undelete-me"},
				"content": []any{"Content"},
			},
		}

		url, _, _ := store.Create(context.Background(), doc)

		store.Delete(context.Background(), url)

		newURL, changed, err := store.Undelete(context.Background(), url)
		if err != nil {
			t.Fatalf("Undelete: %v", err)
		}
		if changed {
			t.Error("URL should not change for undelete")
		}
		if newURL != url {
			t.Errorf("url changed unexpectedly: %s != %s", newURL, url)
		}

		fetched, _ := store.Get(context.Background(), url)
		if _, hasDeleted := fetched.Properties["deleted"]; hasDeleted {
			t.Error("deleted property should be removed")
		}
	})
}

func TestFilesystemContentStore_Get(t *testing.T) {
	t.Run("retrieves existing document", func(t *testing.T) {
		store, _, cleanup := setupFilesystemTest(t)
		defer cleanup()

		doc := util.Mf2Document{
			Type: []string{"h-entry"},
			Properties: map[string][]any{
				"slug":    []any{"get-test"},
				"content": []any{"Test content"},
			},
		}

		url, _, _ := store.Create(context.Background(), doc)

		fetched, err := store.Get(context.Background(), url)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}

		contentProp := fetched.Properties["content"]
		if len(contentProp) == 0 {
			t.Fatal("missing content property")
		}
		content := contentProp[0].(string)
		if content != "Test content" {
			t.Errorf("content = %q, want %q", content, "Test content")
		}
	})

	t.Run("returns ErrNotFound for missing document", func(t *testing.T) {
		store, _, cleanup := setupFilesystemTest(t)
		defer cleanup()

		_, err := store.Get(context.Background(), "https://example.com/missing")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestFilesystemContentStore_ExistsBySlug(t *testing.T) {
	t.Run("returns true for existing slug", func(t *testing.T) {
		store, _, cleanup := setupFilesystemTest(t)
		defer cleanup()

		doc := util.Mf2Document{
			Type: []string{"h-entry"},
			Properties: map[string][]any{
				"slug":    []any{"exists"},
				"content": []any{"Content"},
			},
		}

		store.Create(context.Background(), doc)

		exists, err := store.ExistsBySlug(context.Background(), "exists")
		if err != nil {
			t.Fatalf("ExistsBySlug: %v", err)
		}
		if !exists {
			t.Error("expected exists=true")
		}
	})

	t.Run("returns false for missing slug", func(t *testing.T) {
		store, _, cleanup := setupFilesystemTest(t)
		defer cleanup()

		exists, err := store.ExistsBySlug(context.Background(), "missing")
		if err != nil {
			t.Fatalf("ExistsBySlug: %v", err)
		}
		if exists {
			t.Error("expected exists=false")
		}
	})
}

func TestFilesystemContentStore_ConcurrentAccess(t *testing.T) {
	t.Run("handles concurrent creates", func(t *testing.T) {
		store, _, cleanup := setupFilesystemTest(t)
		defer cleanup()

		done := make(chan struct{})

		for i := 0; i < 5; i++ {
			go func(n int) {
				doc := util.Mf2Document{
					Type: []string{"h-entry"},
					Properties: map[string][]any{
						"slug":    []any{fmt.Sprintf("concurrent-%d", n)},
						"content": []any{fmt.Sprintf("Content %d", n)},
					},
				}

				_, _, err := store.Create(context.Background(), doc)
				if err != nil {
					t.Errorf("Create %d: %v", n, err)
				}
				done <- struct{}{}
			}(i)
		}

		for i := 0; i < 5; i++ {
			<-done
		}
	})
}
