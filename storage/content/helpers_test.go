package content

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/indieinfra/scribble/server/util"
	storageutil "github.com/indieinfra/scribble/storage/util"
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
		if got := storageutil.NormalizeBaseURL(tc.in); got != tc.want {
			t.Fatalf("storageutil.NormalizeBaseURL(%q) = %q, want %q", tc.in, got, tc.want)
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

func TestShouldRecomputeSlug(t *testing.T) {
	cases := []struct {
		name         string
		replacements map[string][]any
		additions    map[string][]any
		want         bool
	}{
		{
			name:         "direct slug replacement",
			replacements: map[string][]any{"slug": []any{"new-slug"}},
			additions:    map[string][]any{},
			want:         true,
		},
		{
			name:         "name replacement",
			replacements: map[string][]any{"name": []any{"New Title"}},
			additions:    map[string][]any{},
			want:         true,
		},
		{
			name:         "content replacement",
			replacements: map[string][]any{"content": []any{"New content"}},
			additions:    map[string][]any{},
			want:         true,
		},
		{
			name:         "name addition",
			replacements: map[string][]any{},
			additions:    map[string][]any{"name": []any{"Added Title"}},
			want:         true,
		},
		{
			name:         "content addition",
			replacements: map[string][]any{},
			additions:    map[string][]any{"content": []any{"Added content"}},
			want:         true,
		},
		{
			name:         "unrelated property",
			replacements: map[string][]any{"category": []any{"test"}},
			additions:    map[string][]any{"syndication": []any{"https://example.com"}},
			want:         false,
		},
		{
			name:         "empty mutations",
			replacements: map[string][]any{},
			additions:    map[string][]any{},
			want:         false,
		},
		{
			name:         "name replaced with empty array",
			replacements: map[string][]any{"name": []any{}},
			additions:    map[string][]any{},
			want:         false,
		},
		{
			name:         "content replaced with empty array",
			replacements: map[string][]any{"content": []any{}},
			additions:    map[string][]any{},
			want:         false,
		},
		{
			name:         "slug replaced with empty array",
			replacements: map[string][]any{"slug": []any{}},
			additions:    map[string][]any{},
			want:         false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldRecomputeSlug(tc.replacements, tc.additions)
			if got != tc.want {
				t.Fatalf("shouldRecomputeSlug() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestComputeNewSlug(t *testing.T) {
	cases := []struct {
		name         string
		doc          *util.Mf2Document
		replacements map[string][]any
		want         string
		wantErr      bool
	}{
		{
			name: "explicit slug replacement",
			doc: &util.Mf2Document{
				Type:       []string{"h-entry"},
				Properties: map[string][]any{"name": []any{"Original Title"}},
			},
			replacements: map[string][]any{"slug": []any{"custom-slug"}},
			want:         "custom-slug",
			wantErr:      false,
		},
		{
			name: "generate from name after replacement",
			doc: &util.Mf2Document{
				Type:       []string{"h-entry"},
				Properties: map[string][]any{"name": []any{"New Amazing Title"}},
			},
			replacements: map[string][]any{},
			want:         "new-amazing-title",
			wantErr:      false,
		},
		{
			name: "generate from content",
			doc: &util.Mf2Document{
				Type:       []string{"h-entry"},
				Properties: map[string][]any{"content": []any{"Hello world this is content"}},
			},
			replacements: map[string][]any{},
			want:         "hello-world-this-is-content",
			wantErr:      false,
		},
		{
			name: "empty slug replacement error",
			doc: &util.Mf2Document{
				Type:       []string{"h-entry"},
				Properties: map[string][]any{},
			},
			replacements: map[string][]any{"slug": []any{""}},
			want:         "",
			wantErr:      true,
		},
		{
			name: "no content to generate slug",
			doc: &util.Mf2Document{
				Type:       []string{"h-entry"},
				Properties: map[string][]any{},
			},
			replacements: map[string][]any{},
			want:         "",
			wantErr:      true,
		},
		{
			name: "empty slug array replacement",
			doc: &util.Mf2Document{
				Type:       []string{"h-entry"},
				Properties: map[string][]any{"name": []any{"Title"}},
			},
			replacements: map[string][]any{"slug": []any{}},
			want:         "",
			wantErr:      true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := computeNewSlug(tc.doc, tc.replacements)
			if (err != nil) != tc.wantErr {
				t.Fatalf("computeNewSlug() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("computeNewSlug() = %q, want %q", got, tc.want)
			}
			// Verify error message is descriptive when expected
			if tc.wantErr && err != nil {
				errMsg := err.Error()
				if !strings.Contains(errMsg, "slug") && !strings.Contains(errMsg, "name") && !strings.Contains(errMsg, "content") {
					t.Fatalf("expected descriptive error message, got: %s", errMsg)
				}
			}
		})
	}
}

// mockStoreForCollisionTest implements ContentStore for testing ensureUniqueSlug
type mockStoreForCollisionTest struct {
	existingSlugs map[string]bool
}

func (m *mockStoreForCollisionTest) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	return m.existingSlugs[slug], nil
}

func (m *mockStoreForCollisionTest) Create(ctx context.Context, doc util.Mf2Document) (string, bool, error) {
	return "", false, nil
}

func (m *mockStoreForCollisionTest) Update(ctx context.Context, url string, replacements map[string][]any, additions map[string][]any, deletions any) (string, error) {
	return "", nil
}

func (m *mockStoreForCollisionTest) Delete(ctx context.Context, url string) error {
	return nil
}

func (m *mockStoreForCollisionTest) Undelete(ctx context.Context, url string) (string, bool, error) {
	return "", false, nil
}

func (m *mockStoreForCollisionTest) Get(ctx context.Context, url string) (*util.Mf2Document, error) {
	return nil, nil
}

func TestEnsureUniqueSlug(t *testing.T) {
	ctx := context.Background()

	t.Run("no collision - slug unchanged", func(t *testing.T) {
		store := &mockStoreForCollisionTest{
			existingSlugs: map[string]bool{"existing-slug": true},
		}

		// If old and new slug are the same, should return as-is
		result, err := ensureUniqueSlug(ctx, store, "same-slug", "same-slug")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "same-slug" {
			t.Fatalf("expected same-slug, got %s", result)
		}
	})

	t.Run("no collision - new slug available", func(t *testing.T) {
		store := &mockStoreForCollisionTest{
			existingSlugs: map[string]bool{"existing-slug": true},
		}

		result, err := ensureUniqueSlug(ctx, store, "new-slug", "old-slug")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "new-slug" {
			t.Fatalf("expected new-slug, got %s", result)
		}
	})

	t.Run("collision - UUID suffix appended", func(t *testing.T) {
		store := &mockStoreForCollisionTest{
			existingSlugs: map[string]bool{"colliding-slug": true},
		}

		result, err := ensureUniqueSlug(ctx, store, "colliding-slug", "old-slug")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should have UUID appended
		if result == "colliding-slug" {
			t.Fatalf("expected UUID suffix to be appended")
		}

		if !strings.HasPrefix(result, "colliding-slug-") {
			t.Fatalf("expected result to start with 'colliding-slug-', got %s", result)
		}

		// Verify UUID format (rough check - should have hyphens)
		suffix := strings.TrimPrefix(result, "colliding-slug-")
		if len(suffix) < 30 || !strings.Contains(suffix, "-") {
			t.Fatalf("expected UUID suffix, got %s", suffix)
		}
	})
}
