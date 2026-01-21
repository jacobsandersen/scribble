package post

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/indieinfra/scribble/server/auth"
	"github.com/indieinfra/scribble/server/body"
	"github.com/indieinfra/scribble/server/util"
)

type stubStore struct{ exists bool }

func (s *stubStore) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	return s.exists, nil
}
func (s *stubStore) Create(context.Context, util.Mf2Document) (string, bool, error) {
	return "", false, nil
}
func (s *stubStore) Update(context.Context, string, map[string][]any, map[string][]any, any) (string, error) {
	return "", nil
}
func (s *stubStore) Delete(context.Context, string) error                   { return nil }
func (s *stubStore) Undelete(context.Context, string) (string, bool, error) { return "", false, nil }
func (s *stubStore) Get(context.Context, string) (*util.Mf2Document, error) { return nil, nil }

type stubMediaStoreErr struct{ err error }

func (s *stubMediaStoreErr) Upload(context.Context, *multipart.File, *multipart.FileHeader) (string, error) {
	return "", s.err
}
func (s *stubMediaStoreErr) Delete(context.Context, string) error { return nil }

func TestDeriveSuggestedSlug(t *testing.T) {
	t.Run("mp-slug wins", func(t *testing.T) {
		doc := util.Mf2Document{Properties: map[string][]any{"mp-slug": []any{"custom"}}}
		if got := deriveSuggestedSlug(&doc); got != "custom" {
			t.Fatalf("expected mp-slug, got %q", got)
		}
	})

	t.Run("generated slug", func(t *testing.T) {
		doc := util.Mf2Document{Properties: map[string][]any{"name": []any{"Hello"}}}
		if got := deriveSuggestedSlug(&doc); got != "hello" {
			t.Fatalf("expected generated slug, got %q", got)
		}
	})

	t.Run("uuid fallback", func(t *testing.T) {
		doc := util.Mf2Document{Properties: map[string][]any{"photo": []any{"noop"}}}
		got := deriveSuggestedSlug(&doc)
		if got == "" {
			t.Fatalf("expected uuid fallback slug")
		}
	})
}

func TestBuildDocumentUnsupportedContentType(t *testing.T) {
	if _, err := buildDocument("text/plain", map[string]any{}); err == nil {
		t.Fatalf("expected unsupported content type to error")
	}
}

func TestNormalizeJsonVariants(t *testing.T) {
	input := map[string]any{
		"type": "h-card",
		"properties": map[string]any{
			"name":      "Alice",
			"category":  []any{"go", map[string]any{"html": []any{"<b>hi</b>"}}, nil, 123, true},
			"note":      map[string]any{"html": "<p>note</p>"},
			"skip-nil":  nil,
			"zero-bool": false,
		},
	}

	doc := normalizeJson(input)

	if got := doc.Type[0]; got != "h-card" {
		t.Fatalf("expected type from json, got %q", got)
	}
	cat := doc.Properties["category"]
	if len(cat) != 4 {
		t.Fatalf("expected nils skipped and mixed values preserved, got %#v", cat)
	}
	if note, ok := doc.Properties["note"]; !ok || len(note) != 1 {
		t.Fatalf("expected note map to be preserved, got %#v", note)
	}
	if _, exists := doc.Properties["skip-nil"]; exists {
		t.Fatalf("expected nil property to be omitted")
	}
	if zero, ok := doc.Properties["zero-bool"]; !ok || len(zero) != 1 || zero[0] != false {
		t.Fatalf("expected bool to be preserved, got %#v", zero)
	}
}

func TestEnsureUniqueSlug(t *testing.T) {
	t.Run("returns slug when unique", func(t *testing.T) {
		store := &stubStore{exists: false}
		got, err := ensureUniqueSlug(context.Background(), store, "slug")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "slug" {
			t.Fatalf("expected slug unchanged, got %q", got)
		}
	})

	t.Run("adds suffix when exists", func(t *testing.T) {
		store := &stubStore{exists: true}
		got, err := ensureUniqueSlug(context.Background(), store, "slug")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == "slug" {
			t.Fatalf("expected slug to change when collision")
		}
		if len(got) <= len("slug") {
			t.Fatalf("expected suffix added, got %q", got)
		}
	})
}

func TestNormalizeFormBodyAndProcessMpProperties(t *testing.T) {
	doc := normalizeFormBody(map[string]any{
		"h":          "entry",
		"category[]": []any{"go", "micropub"},
		"mp-slug":    "custom-slug",
		"skip":       []any{},
	})

	if got := doc.Type[0]; got != "h-entry" {
		t.Fatalf("expected type h-entry, got %q", got)
	}
	if vals := doc.Properties["category"]; len(vals) != 2 {
		t.Fatalf("expected two category values, got %v", vals)
	}

	slug := processMpProperties(&doc)
	if slug != "custom-slug" {
		t.Fatalf("expected mp-slug to be returned, got %q", slug)
	}
	if _, exists := doc.Properties["mp-slug"]; exists {
		t.Fatalf("expected mp-* properties to be removed")
	}
	if _, exists := doc.Properties["skip"]; exists {
		t.Fatalf("expected empty property to be dropped")
	}
}

func TestFirstString(t *testing.T) {
	if s, ok := firstString("hello"); !ok || s != "hello" {
		t.Fatalf("expected to extract string")
	}
	if s, ok := firstString([]any{"world", "extra"}); !ok || s != "world" {
		t.Fatalf("expected to extract first string from slice")
	}
	if _, ok := firstString(123); ok {
		t.Fatalf("expected non-string to return false")
	}
}

func TestCoerceSlice(t *testing.T) {
	vals := coerceSlice([]any{"a", nil, "b"})
	if len(vals) != 2 {
		t.Fatalf("expected nils to be removed")
	}
	vals = coerceSlice("solo")
	if len(vals) != 1 || vals[0] != "solo" {
		t.Fatalf("expected single value to be preserved")
	}
}

func TestMediaPropertyForUpload(t *testing.T) {
	header := &multipart.FileHeader{Header: textproto.MIMEHeader{"Content-Type": []string{"video/mp4"}}}
	if prop := mediaPropertyForUpload(header); prop != "video" {
		t.Fatalf("expected video property, got %q", prop)
	}

	header = &multipart.FileHeader{Header: textproto.MIMEHeader{"Content-Type": []string{"audio/mpeg"}}}
	if prop := mediaPropertyForUpload(header); prop != "audio" {
		t.Fatalf("expected audio property, got %q", prop)
	}

	header = &multipart.FileHeader{Filename: "image.jpg", Header: textproto.MIMEHeader{}}
	if prop := mediaPropertyForUpload(header); prop != "photo" {
		t.Fatalf("expected photo fallback, got %q", prop)
	}

	if prop := mediaPropertyForUpload(nil); prop != "photo" {
		t.Fatalf("expected photo default when header nil, got %q", prop)
	}
}

func TestCreateSlugLookupError(t *testing.T) {
	st := newState()
	cs := &stubContentStore{existsErr: errors.New("boom"), forbidCreate: true}
	st.ContentStore = cs
	st.MediaStore = &stubMediaStore{}

	payload := map[string]any{
		"type":       []any{"h-entry"},
		"properties": map[string]any{"name": []any{"Hello"}},
	}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.AddToken(req.Context(), &auth.TokenDetails{Me: st.Cfg.Micropub.MeUrl, Scope: "create"}))

	rr := httptest.NewRecorder()
	Create(st, rr, req, &body.ParsedBody{Data: map[string]any{"type": []any{"h-entry"}, "properties": map[string]any{"name": []any{"Hello"}}}})

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when slug lookup fails, got %d", rr.Code)
	}
	if cs.createCalled {
		t.Fatalf("expected content create not to be called on slug error")
	}
}

func TestCreateMultipartAccepted(t *testing.T) {
	st := newState()
	cs := &stubContentStore{createURL: "https://example.org/pending", createNow: false}
	st.ContentStore = cs
	st.MediaStore = &stubMediaStore{}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("h", "entry")
	_ = w.WriteField("name", "Photo post")
	_ = w.WriteField("access_token", "bodytoken")
	fw, _ := w.CreateFormFile("photo", "pic.jpg")
	_, _ = fw.Write([]byte("data"))
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req = req.WithContext(auth.AddToken(req.Context(), &auth.TokenDetails{Me: st.Cfg.Micropub.MeUrl, Scope: "create"}))

	rr := httptest.NewRecorder()
	parsed, ok := body.ReadBody(st.Cfg, rr, req)
	if !ok {
		t.Fatalf("expected body to parse")
	}
	Create(st, rr, req, parsed)
	for _, pf := range parsed.Files {
		pf.File.Close()
	}

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
	if rr.Header().Get("Location") != "https://example.org/pending" {
		t.Fatalf("expected Location header set")
	}
	if !cs.createCalled {
		t.Fatalf("expected create to be called")
	}
	photoVals := cs.lastDoc.Properties["photo"]
	if len(photoVals) == 0 {
		t.Fatalf("expected photo property to be set")
	}
	if token := parsed.AccessToken; token != "bodytoken" {
		t.Fatalf("expected access token to be popped from body, got %q", token)
	}
}

func TestCreateUploadError(t *testing.T) {
	st := newState()
	cs := &stubContentStore{forbidCreate: true}
	st.ContentStore = cs
	st.MediaStore = &stubMediaStoreErr{err: errors.New("upload failed")}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("h", "entry")
	fw, _ := w.CreateFormFile("photo", "pic.jpg")
	_, _ = fw.Write([]byte("data"))
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req = req.WithContext(auth.AddToken(req.Context(), &auth.TokenDetails{Me: st.Cfg.Micropub.MeUrl, Scope: "create"}))

	rr := httptest.NewRecorder()
	parsed, ok := body.ReadBody(st.Cfg, rr, req)
	if !ok {
		t.Fatalf("expected body to parse")
	}
	Create(st, rr, req, parsed)
	for _, pf := range parsed.Files {
		pf.File.Close()
	}

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when upload fails, got %d", rr.Code)
	}
	if cs.createCalled {
		t.Fatalf("expected create not to be called on upload failure")
	}
}

func TestCreateContentStoreError(t *testing.T) {
	st := newState()
	cs := &stubContentStore{createErr: errors.New("store boom")}
	st.ContentStore = cs
	st.MediaStore = &stubMediaStore{}

	payload := map[string]any{
		"type":       []any{"h-entry"},
		"properties": map[string]any{"name": []any{"Hello"}},
	}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.AddToken(req.Context(), &auth.TokenDetails{Me: st.Cfg.Micropub.MeUrl, Scope: "create"}))

	rr := httptest.NewRecorder()
	parsed, ok := body.ReadBody(st.Cfg, rr, req)
	if !ok {
		t.Fatalf("expected body to parse")
	}
	Create(st, rr, req, parsed)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when content store fails, got %d", rr.Code)
	}
}
