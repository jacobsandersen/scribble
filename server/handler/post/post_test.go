package post

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/auth"
	"github.com/indieinfra/scribble/server/state"
	"github.com/indieinfra/scribble/server/util"
)

type stubContentStore struct {
	exists       bool
	existsErr    error
	createCalled bool
	lastDoc      util.Mf2Document
	createURL    string
	createNow    bool
	forbidCreate bool
	createErr    error
}

func (s *stubContentStore) ExistsBySlug(_ context.Context, _ string) (bool, error) {
	if s.existsErr != nil {
		return false, s.existsErr
	}
	return s.exists, nil
}
func (s *stubContentStore) Create(_ context.Context, doc util.Mf2Document) (string, bool, error) {
	if s.forbidCreate {
		panic("Create should not be called")
	}
	if s.createErr != nil {
		return "", false, s.createErr
	}
	s.createCalled = true
	s.lastDoc = doc
	if s.createURL == "" {
		s.createURL = "https://example.org/post"
	}
	return s.createURL, s.createNow, nil
}
func (s *stubContentStore) Update(context.Context, string, map[string][]any, map[string][]any, any) (string, error) {
	return "", nil
}
func (s *stubContentStore) Delete(context.Context, string) error { return nil }
func (s *stubContentStore) Undelete(context.Context, string) (string, bool, error) {
	return "", false, nil
}
func (s *stubContentStore) Get(context.Context, string) (*util.Mf2Document, error) { return nil, nil }

type stubMediaStore struct{}

func (s *stubMediaStore) Upload(context.Context, *multipart.File, *multipart.FileHeader) (string, error) {
	return "https://media.example.org/file", nil
}
func (s *stubMediaStore) Delete(context.Context, string) error { return nil }

func newState() *state.ScribbleState {
	return &state.ScribbleState{Cfg: &config.Config{
		Server:   config.Server{Limits: config.ServerLimits{MaxPayloadSize: 2_000_000, MaxFileSize: 1_000_000, MaxMultipartMem: 2_000_000}},
		Micropub: config.Micropub{MeUrl: "https://example.org"},
	}}
}

func TestDispatchPost_TokenConflictHeaderAndBody(t *testing.T) {
	st := newState()
	st.ContentStore = &stubContentStore{forbidCreate: true}
	st.MediaStore = &stubMediaStore{}

	body := "h=entry&name=Test&access_token=bodytoken"
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	tok := &auth.TokenDetails{Me: st.Cfg.Micropub.MeUrl, Scope: "create"}
	req = req.WithContext(auth.AddToken(req.Context(), tok))

	rr := httptest.NewRecorder()
	handler := DispatchPost(st)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for token conflict, got %d", rr.Code)
	}

	if st.ContentStore.(*stubContentStore).createCalled {
		t.Fatalf("content store should not be called on token conflict")
	}
}

func TestDispatchPost_CreateJSONSuccess(t *testing.T) {
	st := newState()
	cs := &stubContentStore{createNow: true}
	st.ContentStore = cs
	st.MediaStore = &stubMediaStore{}

	payload := map[string]any{
		"type":       []any{"h-entry"},
		"properties": map[string]any{"name": []any{"Hello World"}},
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.AddToken(req.Context(), &auth.TokenDetails{Me: st.Cfg.Micropub.MeUrl, Scope: "create"}))

	rr := httptest.NewRecorder()
	handler := DispatchPost(st)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	if !cs.createCalled {
		t.Fatalf("expected content store create to be called")
	}

	slugVals := cs.lastDoc.Properties["slug"]
	if len(slugVals) == 0 || slugVals[0] != "hello-world" {
		t.Fatalf("expected slug 'hello-world', got %#v", slugVals)
	}
}

func TestDispatchPost_UnknownAction(t *testing.T) {
	st := newState()
	st.ContentStore = &stubContentStore{}
	st.MediaStore = &stubMediaStore{}

	payload := map[string]any{"action": "bogus"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.AddToken(req.Context(), &auth.TokenDetails{Me: st.Cfg.Micropub.MeUrl, Scope: "create"}))

	rr := httptest.NewRecorder()
	DispatchPost(st).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown action, got %d", rr.Code)
	}
}

func TestDispatchPost_ActionMustBeString(t *testing.T) {
	st := newState()
	st.ContentStore = &stubContentStore{forbidCreate: true}
	st.MediaStore = &stubMediaStore{}

	payload := map[string]any{"action": 123}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.AddToken(req.Context(), &auth.TokenDetails{Me: st.Cfg.Micropub.MeUrl, Scope: "create"}))

	rr := httptest.NewRecorder()
	DispatchPost(st).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-string action, got %d", rr.Code)
	}
}
