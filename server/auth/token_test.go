package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/indieinfra/scribble/config"
)

func TestTokenDetailsString(t *testing.T) {
	details := &TokenDetails{Me: "https://example.org", ClientId: "client", Scope: "create", IssuedAt: 10, Nonce: 5}
	got := details.String()
	want := "TokenDetails{me=https://example.org, clientId=client, scope=create, issuedAt=10, nonce=5}"

	if got != want {
		t.Fatalf("unexpected String(): %q", got)
	}
}

func TestTokenDetailsHasMe(t *testing.T) {
	cases := []struct {
		name string
		d    TokenDetails
		me   string
		ok   bool
	}{
		{name: "exact match", d: TokenDetails{Me: "https://example.org"}, me: "https://example.org", ok: true},
		{name: "ignore case and spaces", d: TokenDetails{Me: " https://Example.org/ "}, me: "https://example.org/", ok: true},
		{name: "mismatch", d: TokenDetails{Me: "https://other"}, me: "https://example.org", ok: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.d.HasMe(tc.me); got != tc.ok {
				t.Fatalf("HasMe() = %v, want %v", got, tc.ok)
			}
		})
	}
}

func TestVerifyAccessToken_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		_ = json.NewEncoder(w).Encode(TokenDetails{
			Me:       "https://example.org",
			ClientId: "client",
			Scope:    "create",
			IssuedAt: 1,
			Nonce:    0,
		})
	}))
	defer srv.Close()

	cfg := &config.Config{Micropub: config.Micropub{MeUrl: "https://example.org", TokenEndpoint: srv.URL}}

	details := VerifyAccessToken(cfg, "ok")
	if details == nil {
		t.Fatalf("expected token details, got nil")
	}
	if details.ClientId != "client" || details.Scope != "create" {
		t.Fatalf("unexpected token details: %+v", details)
	}
}

func TestVerifyAccessToken_InvalidStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	cfg := &config.Config{Micropub: config.Micropub{MeUrl: "https://example.org", TokenEndpoint: srv.URL}}

	if details := VerifyAccessToken(cfg, "bad"); details != nil {
		t.Fatalf("expected nil details for invalid token")
	}
}

func TestVerifyAccessToken_MismatchedMe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(TokenDetails{Me: "https://other.example"})
	}))
	defer srv.Close()

	cfg := &config.Config{Micropub: config.Micropub{MeUrl: "https://example.org", TokenEndpoint: srv.URL}}

	if details := VerifyAccessToken(cfg, "ok"); details != nil {
		t.Fatalf("expected nil details when me does not match")
	}
}

func TestVerifyAccessToken_PanicsOnEmptyToken(t *testing.T) {
	cfg := &config.Config{Micropub: config.Micropub{TokenEndpoint: "https://example.org"}}

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for empty token")
		}
	}()

	_ = VerifyAccessToken(cfg, "")
}

func TestAddAndGetToken(t *testing.T) {
	token := &TokenDetails{Scope: "read"}
	ctx := AddToken(context.Background(), token)

	if got := GetToken(ctx); got != token {
		t.Fatalf("expected token to round-trip via context")
	}
}

func TestTokenDetailsHasScope(t *testing.T) {
	details := TokenDetails{Scope: "read create"}

	if !details.HasScope(ScopeCreate) {
		t.Fatalf("expected HasScope to find create")
	}

	if details.HasScope(ScopeMedia) {
		t.Fatalf("did not expect HasScope to match missing scope")
	}
}
