package integration

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/handler/get"
	"github.com/indieinfra/scribble/server/handler/post"
	"github.com/indieinfra/scribble/server/handler/upload"
	"github.com/indieinfra/scribble/server/state"
	"github.com/indieinfra/scribble/server/util"
	"github.com/indieinfra/scribble/storage/content"
	"github.com/indieinfra/scribble/storage/media"
)

func newFilesystemState(tb testing.TB) *state.ScribbleState {
	tb.Helper()

	contentDir := tb.TempDir()
	mediaDir := tb.TempDir()

	cfg := &config.Config{
		Debug:  false,
		Server: config.Server{Limits: config.ServerLimits{MaxPayloadSize: 1 << 20, MaxFileSize: 1 << 20, MaxMultipartMem: 1 << 20}},
		Micropub: config.Micropub{
			MeUrl:         "https://example.test/me",
			TokenEndpoint: "https://example.test/token",
		},
		Content: config.Content{
			Strategy: "filesystem",
			Filesystem: &config.FilesystemContentStrategy{
				Path:        contentDir,
				PublicUrl:   "https://example.test/content/",
				PathPattern: "{slug}.json",
			},
		},
		Media: config.Media{
			Strategy: "filesystem",
			Filesystem: &config.FilesystemMediaStrategy{
				Path:        mediaDir,
				PublicUrl:   "https://example.test/media/",
				PathPattern: "{year}/{month}/{filename}",
			},
		},
	}

	contentStore, err := content.NewFilesystemContentStore(cfg.Content.Filesystem)
	if err != nil {
		tb.Fatalf("failed to create filesystem content store: %v", err)
	}

	mediaStore, err := media.NewFilesystemMediaStore(cfg.Media.Filesystem)
	if err != nil {
		tb.Fatalf("failed to create filesystem media store: %v", err)
	}

	return &state.ScribbleState{
		Cfg:          cfg,
		ContentStore: contentStore,
		MediaStore:   mediaStore,
	}
}

func TestFilesystem_BasicCreateAndRetrieve(t *testing.T) {
	st := newFilesystemState(t)

	// Create a post
	createBody := map[string]any{
		"type": []string{"h-entry"},
		"properties": map[string][]any{
			"name":    {"Test Post"},
			"content": {"Test content"},
		},
	}

	body, _ := json.Marshal(createBody)
	req := httptest.NewRequest("POST", "/micropub", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	location := rec.Header().Get("Location")

	// Retrieve the post
	req = httptest.NewRequest("GET", "/micropub?q=source&url="+location, nil)
	rec = httptest.NewRecorder()
	withToken(st.Cfg, get.DispatchGet(st)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var retrieved util.Mf2Document
	json.NewDecoder(rec.Body).Decode(&retrieved)

	if len(retrieved.Properties["name"]) == 0 {
		t.Fatal("expected name property")
	}
	name := retrieved.Properties["name"][0].(string)
	if name != "Test Post" {
		t.Errorf("expected 'Test Post', got %q", name)
	}
}

func TestFilesystem_MediaUpload(t *testing.T) {
	st := newFilesystemState(t)

	// Create multipart upload
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="test.jpg"`)
	h.Set("Content-Type", "image/jpeg")

	part, _ := writer.CreatePart(h)
	part.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0}) // JPEG header
	part.Write([]byte("fake image"))

	writer.Close()

	req := httptest.NewRequest("POST", "/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rec := httptest.NewRecorder()
	withToken(st.Cfg, upload.HandleMediaUpload(st)).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("expected Location header")
	}

	// Verify file exists on disk
	mediaDir := st.Cfg.Media.Filesystem.Path
	found := false
	filepath.Walk(mediaDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Ext(path) == ".jpg" {
			found = true
		}
		return nil
	})

	if !found {
		t.Error("uploaded file not found on filesystem")
	}
}

func TestFilesystem_SlugCollision(t *testing.T) {
	st := newFilesystemState(t)

	// Create first post with explicit slug
	createBody := map[string]any{
		"type": []string{"h-entry"},
		"properties": map[string][]any{
			"mp-slug": {"my-slug"},
			"name":    {"First"},
			"content": {"First content"},
		},
	}

	body, _ := json.Marshal(createBody)
	req := httptest.NewRequest("POST", "/micropub", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)
	firstLocation := rec.Header().Get("Location")

	// Create second post with same slug
	createBody["properties"].(map[string][]any)["name"] = []any{"Second"}
	body, _ = json.Marshal(createBody)
	req = httptest.NewRequest("POST", "/micropub", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec = httptest.NewRecorder()
	withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)
	secondLocation := rec.Header().Get("Location")

	// Verify different locations (UUID appended to second)
	if firstLocation == secondLocation {
		t.Error("expected different locations for colliding slugs")
	}
}

func TestFilesystem_PathPattern(t *testing.T) {
	contentDir := t.TempDir()

	cfg := &config.Config{
		Debug:  false,
		Server: config.Server{Limits: config.ServerLimits{MaxPayloadSize: 1 << 20, MaxFileSize: 1 << 20, MaxMultipartMem: 1 << 20}},
		Micropub: config.Micropub{
			MeUrl:         "https://example.test/me",
			TokenEndpoint: "https://example.test/token",
		},
		Content: config.Content{
			Strategy: "filesystem",
			Filesystem: &config.FilesystemContentStrategy{
				Path:        contentDir,
				PublicUrl:   "https://example.test/content/",
				PathPattern: "{year}/{month}/{slug}.json",
			},
		},
		Media: config.Media{Strategy: "noop"},
	}

	contentStore, _ := content.NewFilesystemContentStore(cfg.Content.Filesystem)

	st := &state.ScribbleState{
		Cfg:          cfg,
		ContentStore: contentStore,
		MediaStore:   &media.NoopMediaStore{},
	}

	// Create a post
	createBody := map[string]any{
		"type": []string{"h-entry"},
		"properties": map[string][]any{
			"name":    {"Test"},
			"content": {"Content"},
		},
	}

	body, _ := json.Marshal(createBody)
	req := httptest.NewRequest("POST", "/micropub", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	// Verify file structure
	foundNested := false
	filepath.Walk(contentDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			relPath, _ := filepath.Rel(contentDir, path)
			dir := filepath.Dir(relPath)
			// Check if it's nested (not at root)
			if dir != "." {
				foundNested = true
			}
		}
		return nil
	})

	if !foundNested {
		t.Error("expected nested directory structure from path pattern")
	}
}

// Benchmark tests

func BenchmarkFilesystem_CreatePost(b *testing.B) {
	st := newFilesystemState(b)

	body := `{"type":["h-entry"],"properties":{"name":["Benchmark Post"],"content":["Benchmark content"]}}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/micropub", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			b.Fatalf("expected 201, got %d", rec.Code)
		}
	}
}

func BenchmarkFilesystem_ReadPost(b *testing.B) {
	st := newFilesystemState(b)

	// Create a post first
	createBody := `{"type":["h-entry"],"properties":{"name":["Benchmark Post"],"content":["Benchmark content"]},"mp-slug":["bench-post"]}`
	req := httptest.NewRequest("POST", "/micropub", strings.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)

	location := rec.Header().Get("Location")
	if location == "" {
		b.Fatal("no location header")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/micropub?q=source&url="+location, nil)
		rec := httptest.NewRecorder()
		withToken(st.Cfg, get.DispatchGet(st)).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			b.Fatalf("expected 200, got %d", rec.Code)
		}
	}
}

func BenchmarkFilesystem_PathPatternCreate(b *testing.B) {
	// Create a separate state with a complex path pattern
	contentDir := b.TempDir()
	mediaDir := b.TempDir()

	cfg := &config.Config{
		Debug:  false,
		Server: config.Server{Limits: config.ServerLimits{MaxPayloadSize: 1 << 20, MaxFileSize: 1 << 20, MaxMultipartMem: 1 << 20}},
		Micropub: config.Micropub{
			MeUrl:         "https://example.test/me",
			TokenEndpoint: "https://example.test/token",
		},
		Content: config.Content{
			Strategy: "filesystem",
			Filesystem: &config.FilesystemContentStrategy{
				Path:        contentDir,
				PublicUrl:   "https://example.test/content/",
				PathPattern: "{year}/{month}/{day}/{slug}.json",
			},
		},
		Media: config.Media{
			Strategy: "filesystem",
			Filesystem: &config.FilesystemMediaStrategy{
				Path:        mediaDir,
				PublicUrl:   "https://example.test/media/",
				PathPattern: "{year}/{month}/{filename}",
			},
		},
	}

	contentStore, err := content.NewFilesystemContentStore(cfg.Content.Filesystem)
	if err != nil {
		b.Fatal(err)
	}

	mediaStore, err := media.NewFilesystemMediaStore(cfg.Media.Filesystem)
	if err != nil {
		b.Fatal(err)
	}

	st := &state.ScribbleState{
		Cfg:          cfg,
		ContentStore: contentStore,
		MediaStore:   mediaStore,
	}

	body := `{"type":["h-entry"],"properties":{"name":["Pattern Post"],"content":["Pattern content"]}}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/micropub", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			b.Fatalf("expected 201, got %d", rec.Code)
		}
	}
}

// Additional query endpoint tests

func TestFilesystem_QueryConfig(t *testing.T) {
	st := newFilesystemState(t)

	req := httptest.NewRequest("GET", "/micropub?q=config", nil)
	rec := httptest.NewRecorder()
	withToken(st.Cfg, get.DispatchGet(st)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var config map[string]any
	json.NewDecoder(rec.Body).Decode(&config)

	if config["media-endpoint"] == nil {
		t.Error("expected media-endpoint in config response")
	}

	if config["syndicate-to"] == nil {
		t.Error("expected syndicate-to in config response")
	}
}

func TestFilesystem_QuerySyndicateTo(t *testing.T) {
	st := newFilesystemState(t)

	req := httptest.NewRequest("GET", "/micropub?q=syndicate-to", nil)
	rec := httptest.NewRecorder()
	withToken(st.Cfg, get.DispatchGet(st)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)

	// Should return empty syndicate-to array
	if targets, ok := result["syndicate-to"].([]any); !ok || len(targets) != 0 {
		t.Errorf("expected empty syndicate-to array, got %v", result)
	}
}

func TestFilesystem_UpdateAddProperties(t *testing.T) {
	st := newFilesystemState(t)

	// Create post
	createBody := `{"type":["h-entry"],"properties":{"name":["Test Post"],"content":["Initial content"]}}`
	req := httptest.NewRequest("POST", "/micropub", bytes.NewReader([]byte(createBody)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)

	location := rec.Header().Get("Location")

	// Add category property
	updateBody := `{"action":"update","url":"` + location + `","add":{"category":["test","integration"]}}`
	req = httptest.NewRequest("POST", "/micropub", bytes.NewReader([]byte(updateBody)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for add, got %d", rec.Code)
	}

	// Verify categories were added
	req = httptest.NewRequest("GET", "/micropub?q=source&url="+location, nil)
	rec = httptest.NewRecorder()
	withToken(st.Cfg, get.DispatchGet(st)).ServeHTTP(rec, req)

	var updated util.Mf2Document
	json.NewDecoder(rec.Body).Decode(&updated)

	if len(updated.Properties["category"]) != 2 {
		t.Errorf("expected 2 categories, got %d", len(updated.Properties["category"]))
	}
}

func TestFilesystem_UpdateDeleteProperties(t *testing.T) {
	st := newFilesystemState(t)

	// Create post with multiple properties
	createBody := `{"type":["h-entry"],"properties":{"name":["Test Post"],"content":["Content"],"category":["tag1","tag2"],"syndication":["https://example.com/1"]}}`
	req := httptest.NewRequest("POST", "/micropub", bytes.NewReader([]byte(createBody)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)

	location := rec.Header().Get("Location")

	// Delete entire category property
	updateBody := `{"action":"update","url":"` + location + `","delete":["category"]}`
	req = httptest.NewRequest("POST", "/micropub", bytes.NewReader([]byte(updateBody)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for delete, got %d", rec.Code)
	}

	// Verify category was deleted
	req = httptest.NewRequest("GET", "/micropub?q=source&url="+location, nil)
	rec = httptest.NewRecorder()
	withToken(st.Cfg, get.DispatchGet(st)).ServeHTTP(rec, req)

	var updated util.Mf2Document
	json.NewDecoder(rec.Body).Decode(&updated)

	if _, exists := updated.Properties["category"]; exists {
		t.Error("expected category property to be deleted")
	}
}

func TestFilesystem_QuerySourceWithProperties(t *testing.T) {
	st := newFilesystemState(t)

	// Create post
	createBody := `{"type":["h-entry"],"properties":{"name":["Test Post"],"content":["Content"],"category":["tag1","tag2"]}}`
	req := httptest.NewRequest("POST", "/micropub", bytes.NewReader([]byte(createBody)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	withToken(st.Cfg, post.DispatchPost(st)).ServeHTTP(rec, req)

	location := rec.Header().Get("Location")

	// Query source with specific properties filter - need to properly construct the query string
	req = httptest.NewRequest("GET", "/micropub?q=source&url="+location+"&properties=name&properties=category", nil)
	rec = httptest.NewRecorder()
	withToken(st.Cfg, get.DispatchGet(st)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var filtered util.Mf2Document
	json.NewDecoder(rec.Body).Decode(&filtered)

	// Should only have name and category, not content
	if _, hasName := filtered.Properties["name"]; !hasName {
		t.Error("expected name property in filtered response")
	}
	if _, hasCategory := filtered.Properties["category"]; !hasCategory {
		t.Error("expected category property in filtered response")
	}
	if _, hasContent := filtered.Properties["content"]; hasContent {
		t.Errorf("did not expect content property in filtered response, got properties: %v", filtered.Properties)
	}
}
