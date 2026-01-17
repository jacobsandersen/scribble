package util

import "testing"

func TestGenerateSlug(t *testing.T) {
	t.Run("uses name when present", func(t *testing.T) {
		doc := Mf2Document{Properties: map[string][]any{"name": {"Hello World"}}}
		if slug := GenerateSlug(doc); slug != "hello-world" {
			t.Fatalf("expected slug from name, got %q", slug)
		}
	})

	t.Run("falls back to content", func(t *testing.T) {
		doc := Mf2Document{Properties: map[string][]any{"content": {"An interesting post"}}}
		if slug := GenerateSlug(doc); slug != "an-interesting-post" {
			t.Fatalf("expected slug from content, got %q", slug)
		}
	})

	t.Run("combines name and content when name short", func(t *testing.T) {
		doc := Mf2Document{Properties: map[string][]any{"name": {"Hello"}, "content": {"world from scribble today"}}}
		slug := GenerateSlug(doc)
		if slug != "hello-world-from-scribble-today" {
			t.Fatalf("unexpected slug: %q", slug)
		}
	})

	t.Run("empty when no usable fields", func(t *testing.T) {
		doc := Mf2Document{Properties: map[string][]any{"photo": {"http://example.com/img.jpg"}}}
		if slug := GenerateSlug(doc); slug != "" {
			t.Fatalf("expected empty slug when no name/content, got %q", slug)
		}
	})
}

func TestSlugFromURL(t *testing.T) {
	slug, err := SlugFromURL("https://example.org/posts/hello-world")
	if err != nil {
		t.Fatalf("expected slug, got error %v", err)
	}
	if slug != "hello-world" {
		t.Fatalf("unexpected slug %q", slug)
	}

	slug, err = SlugFromURL("https://example.org/posts/")
	if err != nil {
		t.Fatalf("expected trailing slash to still return slug, got error %v", err)
	}
	if slug != "posts" {
		t.Fatalf("expected slug 'posts', got %q", slug)
	}
}

func TestSlugFromURL_Errors(t *testing.T) {
	if _, err := SlugFromURL(""); err == nil {
		t.Fatalf("expected error for empty url")
	}

	if _, err := SlugFromURL("/"); err == nil {
		t.Fatalf("expected error for root slash")
	}
}

func TestExtractTextFromProperty(t *testing.T) {
	val := extractTextFromProperty([]any{map[string]any{"html": "<p>Hello <b>world</b></p>"}})
	if val == "" {
		t.Fatalf("expected html text to be extracted")
	}

	val = extractTextFromProperty([]any{map[string]any{"html": []any{"<p>Array</p>"}}})
	if val == "" {
		t.Fatalf("expected html array text to be extracted")
	}

	if val := extractTextFromProperty([]any{nil, ""}); val != "" {
		t.Fatalf("expected empty string for nil/empty values, got %q", val)
	}
}
