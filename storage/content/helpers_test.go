package content

import (
	"reflect"
	"testing"

	"github.com/indieinfra/scribble/server/util"
)

func TestDeleteValuesAndContains(t *testing.T) {
	values := []any{"keep", 1, map[string]any{"k": "v"}}
	remaining := deleteValues(values, []any{"keep", map[string]any{"k": "v"}})

	if len(remaining) != 1 || remaining[0] != 1 {
		t.Fatalf("unexpected remaining values: %+v", remaining)
	}

	if !containsValue([]any{map[string]any{"k": "v"}}, map[string]any{"k": "v"}) {
		t.Fatalf("expected containsValue to match deep equal values")
	}
}

func TestApplyMutationsAndDeletedFlag(t *testing.T) {
	doc := &util.Mf2Document{Properties: map[string][]any{
		"slug":   []any{"s"},
		"keep":   []any{"yes"},
		"remove": []any{"x", "y"},
	}}

	replacements := map[string][]any{"keep": []any{"replaced"}}
	additions := map[string][]any{"add": []any{1, 2}}
	deletions := map[string][]any{"remove": []any{"x", "y"}}

	applyMutations(doc, replacements, additions, deletions)

	if got := doc.Properties["keep"]; !reflect.DeepEqual(got, []any{"replaced"}) {
		t.Fatalf("unexpected replacements: %+v", got)
	}
	if got := doc.Properties["add"]; !reflect.DeepEqual(got, []any{1, 2}) {
		t.Fatalf("unexpected additions: %+v", got)
	}
	if _, ok := doc.Properties["remove"]; ok {
		t.Fatalf("expected removed key to be deleted")
	}

	// Delete by key slice branch
	applyMutations(doc, nil, nil, []string{"add"})
	if _, ok := doc.Properties["add"]; ok {
		t.Fatalf("expected add key to be deleted via slice")
	}

	doc.Properties["deleted"] = []any{"true"}
	if !deletedFlag(doc) {
		t.Fatalf("expected string true to be treated as deleted")
	}

	doc.Properties["deleted"] = []any{123}
	if deletedFlag(doc) {
		t.Fatalf("non-bool/string should not be treated as deleted")
	}

	if deletedFlag(nil) {
		t.Fatalf("nil doc should not be deleted")
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://example.com", "https://example.com/"},
		{"https://example.com/", "https://example.com/"},
		{"  https://example.com/api  ", "https://example.com/api/"},
		{"https://example.com/nested//", "https://example.com/nested/"},
	}

	for _, tc := range cases {
		if got := normalizeBaseURL(tc.in); got != tc.want {
			t.Fatalf("normalizeBaseURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestExtractSlug(t *testing.T) {
	good := util.Mf2Document{Type: []string{"h-entry"}, Properties: map[string][]any{"slug": []any{"ok"}}}
	if slug, err := extractSlug(good); err != nil || slug != "ok" {
		t.Fatalf("extractSlug good doc got slug=%q err=%v", slug, err)
	}

	missing := util.Mf2Document{Type: []string{"h-entry"}, Properties: map[string][]any{}}
	if _, err := extractSlug(missing); err == nil {
		t.Fatalf("expected error for missing slug")
	}

	empty := util.Mf2Document{Type: []string{"h-entry"}, Properties: map[string][]any{"slug": []any{""}}}
	if _, err := extractSlug(empty); err == nil {
		t.Fatalf("expected error for empty slug")
	}
}
