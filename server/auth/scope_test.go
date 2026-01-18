package auth

import (
	"net/http/httptest"
	"testing"
)

func TestScopeString(t *testing.T) {
	if ScopeRead.String() != "read" || ScopeMedia.String() != "media" {
		t.Fatalf("unexpected scope string values")
	}
}

func TestRequestHasScope(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	if RequestHasScope(req, ScopeCreate) {
		t.Fatalf("expected false when no token in context")
	}

	req = req.WithContext(AddToken(req.Context(), &TokenDetails{Scope: "create"}))
	if !RequestHasScope(req, ScopeCreate) {
		t.Fatalf("expected true when token has scope")
	}
}
