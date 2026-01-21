package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/indieinfra/scribble/config"
	"github.com/indieinfra/scribble/server/auth"
)

func TestValidateTokenMiddleware_MissingTokenOnGet(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(tokenSrv.Close)

	cfg := &config.Config{Micropub: config.Micropub{MeUrl: "https://example.org", TokenEndpoint: tokenSrv.URL}}

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	ValidateTokenMiddleware(cfg, next).ServeHTTP(rr, req)

	if nextCalled {
		t.Fatalf("next handler should not be called when token missing on GET")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestValidateTokenMiddleware_AllowsPostWithoutHeader(t *testing.T) {
	cfg := &config.Config{Micropub: config.Micropub{MeUrl: "https://example.org", TokenEndpoint: "https://token.example"}}

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)

	ValidateTokenMiddleware(cfg, next).ServeHTTP(rr, req)

	if !called {
		t.Fatalf("expected next handler to run for POST without token")
	}
	if rr.Code != http.StatusCreated {
		t.Fatalf("unexpected status: %d", rr.Code)
	}
}

func TestValidateTokenMiddleware_InvalidToken(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(tokenSrv.Close)

	cfg := &config.Config{Micropub: config.Micropub{MeUrl: "https://example.org", TokenEndpoint: tokenSrv.URL}}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer bad")

	ValidateTokenMiddleware(cfg, next).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestValidateTokenMiddleware_ValidTokenPassesThrough(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(auth.TokenDetails{Me: "https://example.org", Scope: "create"})
	}))
	t.Cleanup(tokenSrv.Close)

	cfg := &config.Config{Micropub: config.Micropub{MeUrl: "https://example.org", TokenEndpoint: tokenSrv.URL}}

	var gotToken *auth.TokenDetails
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = auth.GetToken(r.Context())
		w.WriteHeader(http.StatusAccepted)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer good")

	ValidateTokenMiddleware(cfg, next).ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: %d", rr.Code)
	}
	if gotToken == nil || gotToken.Me != "https://example.org" {
		t.Fatalf("expected token details to be stored in context")
	}
}

func TestEnsureTokenForRequest_UsesExistingContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(auth.AddToken(req.Context(), &auth.TokenDetails{Me: "https://example.org"}))
	rr := httptest.NewRecorder()

	gotReq, ok := EnsureTokenForRequest(&config.Config{}, rr, req, "")
	if !ok {
		t.Fatalf("expected ok when token already present")
	}
	if gotReq != req {
		t.Fatalf("expected request to be returned unchanged")
	}
}

func TestEnsureTokenForRequest_MissingToken(t *testing.T) {
	cfg := &config.Config{}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)

	if r, ok := EnsureTokenForRequest(cfg, rr, req, ""); ok || r != nil {
		t.Fatalf("expected failure for missing token")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestEnsureTokenForRequest_InvalidToken(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(tokenSrv.Close)

	cfg := &config.Config{Micropub: config.Micropub{MeUrl: "https://example.org", TokenEndpoint: tokenSrv.URL}}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)

	if r, ok := EnsureTokenForRequest(cfg, rr, req, "bad"); ok || r != nil {
		t.Fatalf("expected failure for invalid token")
	}
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestEnsureTokenForRequest_ValidToken(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(auth.TokenDetails{Me: "https://example.org"})
	}))
	t.Cleanup(tokenSrv.Close)

	cfg := &config.Config{Micropub: config.Micropub{MeUrl: "https://example.org", TokenEndpoint: tokenSrv.URL}}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)

	gotReq, ok := EnsureTokenForRequest(cfg, rr, req, "good")
	if !ok || gotReq == nil {
		t.Fatalf("expected token validation to succeed")
	}

	if tok := auth.GetToken(gotReq.Context()); tok == nil || tok.Me != "https://example.org" {
		t.Fatalf("expected token to be set in context")
	}
}
