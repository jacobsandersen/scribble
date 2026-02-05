package util

import (
	"strings"
	"testing"

	serverutil "github.com/indieinfra/scribble/server/util"
)

const testNewSlug = "my-uncool-post"

func TestReplaceSlugParam(t *testing.T) {
	pattern := "https://example.org/{year}/{month}/{day}/{slug}"

	pat, err := NewContentPathPattern(pattern)
	if err != nil {
		t.Fatalf("unexpected pattern construction error: %v", err)
	}

	tests := []struct {
		name        string
		urlInst     string
		newSlug     string
		want        string
		errContains string
	}{
		{
			name:    "happy path replaces slug",
			urlInst: "https://example.org/2026/02/04/my-cool-post",
			newSlug: testNewSlug,
			want:    "https://example.org/2026/02/04/my-uncool-post",
		},
		{
			name:        "rejects non-matching host",
			urlInst:     "https://not-example.org/2026/02/04/my-cool-post",
			newSlug:     testNewSlug,
			errContains: "URL does not match the path pattern",
		},
		{
			name:        "rejects empty slug",
			urlInst:     "https://example.org/2026/02/04/my-cool-post",
			newSlug:     "",
			errContains: "slug cannot be empty",
		},
		{
			name:        "rejects when slug slot missing",
			urlInst:     "https://example.org/2026/02/04/",
			newSlug:     testNewSlug,
			errContains: "URL does not match the path pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pat.ReplaceSlugParam(tt.urlInst, tt.newSlug)

			if tt.errContains != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}

			if !serverutil.UrlIsInstance(pattern, got) {
				t.Fatalf("resulting URL %q is not an instance of pattern", got)
			}
		})
	}
}
