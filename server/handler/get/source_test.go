package get

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/indieinfra/scribble/server/state"
	"github.com/indieinfra/scribble/server/util"
	"github.com/indieinfra/scribble/storage/content"
)

type fakeContentStore struct {
	getFn func(ctx context.Context, url string) (*util.Mf2Document, error)
}

func (f *fakeContentStore) Create(context.Context, util.Mf2Document) (string, bool, error) {
	return "", false, nil
}

func (f *fakeContentStore) Update(context.Context, string, map[string][]any, map[string][]any, any) (string, error) {
	return "", nil
}

func (f *fakeContentStore) Delete(context.Context, string) error { return nil }

func (f *fakeContentStore) Undelete(context.Context, string) (string, bool, error) {
	return "", false, nil
}

func (f *fakeContentStore) Get(ctx context.Context, url string) (*util.Mf2Document, error) {
	if f.getFn != nil {
		return f.getFn(ctx, url)
	}
	return nil, errors.New("no getFn provided")
}

func (f *fakeContentStore) ExistsBySlug(context.Context, string) (bool, error) { return false, nil }

func TestHandleSource_Success(t *testing.T) {
	doc := &util.Mf2Document{Type: []string{"h-entry"}, Properties: map[string][]any{"name": []any{"hello"}}}
	st := &state.ScribbleState{ContentStore: &fakeContentStore{getFn: func(ctx context.Context, url string) (*util.Mf2Document, error) {
		return doc, nil
	}}}

	r := httptest.NewRequest(http.MethodGet, "/?q=source&url=https://example.org/post", nil)
	w := httptest.NewRecorder()

	HandleSource(st, w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ctype := resp.Header.Get("Content-Type"); ctype != "application/json" {
		t.Fatalf("expected application/json, got %q", ctype)
	}

	var got util.Mf2Document
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if fmt.Sprint(got) != fmt.Sprint(*doc) {
		t.Fatalf("unexpected response body: %+v", got)
	}
}

func TestHandleSource_NotFound(t *testing.T) {
	st := &state.ScribbleState{ContentStore: &fakeContentStore{getFn: func(ctx context.Context, url string) (*util.Mf2Document, error) {
		return nil, content.ErrNotFound
	}}}

	r := httptest.NewRequest(http.MethodGet, "/?q=source&url=https://example.org/missing", nil)
	w := httptest.NewRecorder()

	HandleSource(st, w, r)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Result().StatusCode)
	}
}

func TestHandleSource_InternalError(t *testing.T) {
	st := &state.ScribbleState{ContentStore: &fakeContentStore{getFn: func(ctx context.Context, url string) (*util.Mf2Document, error) {
		return nil, fmt.Errorf("boom")
	}}}

	r := httptest.NewRequest(http.MethodGet, "/?q=source&url=https://example.org/error", nil)
	w := httptest.NewRecorder()

	HandleSource(st, w, r)

	if w.Result().StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Result().StatusCode)
	}
}

func TestHandleSource_MissingURL(t *testing.T) {
	st := &state.ScribbleState{ContentStore: &fakeContentStore{}}

	r := httptest.NewRequest(http.MethodGet, "/?q=source", nil)
	w := httptest.NewRecorder()

	HandleSource(st, w, r)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Result().StatusCode)
	}
}
