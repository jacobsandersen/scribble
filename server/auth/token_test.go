package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/indieinfra/scribble/config"
)

func TestExtractBearerToken(t *testing.T) {
	cases := []struct {
		name   string
		value  string
		expect string
	}{
		{name: "empty", value: "", expect: ""},
		{name: "no scheme", value: "token", expect: ""},
		{name: "wrong scheme", value: "Basic abc", expect: ""},
		{name: "valid", value: "Bearer abc123", expect: "abc123"},
		{name: "case insensitive", value: "bearer token", expect: "token"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractBearerToken(tc.value); got != tc.expect {
				t.Fatalf("ExtractBearerToken(%q) = %q, want %q", tc.value, got, tc.expect)
			}
		})
	}
}

func TestPopAccessToken(t *testing.T) {
	cases := []struct {
		name     string
		input    map[string]any
		expected string
	}{
		{
			name:     "nil map",
			input:    nil,
			expected: "",
		},
		{
			name:     "no token",
			input:    map[string]any{"foo": "bar"},
			expected: "",
		},
		{
			name:     "string token",
			input:    map[string]any{"access_token": "abc"},
			expected: "abc",
		},
		{
			name:     "slice token picks first string",
			input:    map[string]any{"access_token": []any{"", "token2", "ignored"}},
			expected: "token2",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := PopAccessToken(tc.input)
			if got != tc.expected {
				t.Fatalf("PopAccessToken() = %q, want %q", got, tc.expected)
			}

			if tc.input != nil {
				if _, exists := tc.input["access_token"]; exists {
					t.Fatalf("access_token key should be removed from input map")
				}
			}
		})
	}
}

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
				t.Fatalf("HasMe() = %v, want %v (details=%q, me=%q)", got, tc.ok, tc.d.Me, tc.me)
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

	details, err := VerifyAccessToken(cfg, "ok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

	details, err := VerifyAccessToken(cfg, "bad")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if details != nil {
		t.Fatalf("expected nil details for invalid token")
	}
}

func TestVerifyAccessToken_MismatchedMe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(TokenDetails{Me: "https://other.example"})
	}))
	defer srv.Close()

	cfg := &config.Config{Micropub: config.Micropub{MeUrl: "https://example.org", TokenEndpoint: srv.URL}}

	details, err := VerifyAccessToken(cfg, "ok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if details != nil {
		t.Fatalf("expected nil details when me does not match")
	}
}

func TestVerifyAccessToken_ReturnsErrorOnEmptyToken(t *testing.T) {
	cfg := &config.Config{Micropub: config.Micropub{TokenEndpoint: "https://example.org"}}

	details, err := VerifyAccessToken(cfg, "")
	if err != ErrEmptyToken {
		t.Fatalf("expected ErrEmptyToken, got %v", err)
	}
	if details != nil {
		t.Fatalf("expected nil details for empty token")
	}
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
