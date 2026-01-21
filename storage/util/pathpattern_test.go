package util

import (
	"testing"
	"time"
)

func TestPathPattern_Generate(t *testing.T) {
	testTime := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		pattern   string
		slug      string
		timestamp time.Time
		ext       string
		expected  string
		wantErr   bool
	}{
		{
			name:      "simple slug and extension",
			pattern:   "{slug}{ext}",
			slug:      "my-post",
			timestamp: time.Time{},
			ext:       ".json",
			expected:  "my-post.json",
		},
		{
			name:      "year/month/slug pattern",
			pattern:   "{year}/{month}/{slug}.json",
			slug:      "hello-world",
			timestamp: testTime,
			ext:       "",
			expected:  "2026/01/hello-world.json",
		},
		{
			name:      "full date pattern",
			pattern:   "{year}/{month}/{day}/{filename}",
			slug:      "my-photo",
			timestamp: testTime,
			ext:       ".jpg",
			expected:  "2026/01/15/my-photo.jpg",
		},
		{
			name:      "extension without leading dot",
			pattern:   "{slug}{ext}",
			slug:      "test",
			timestamp: time.Time{},
			ext:       "json",
			expected:  "test.json",
		},
		{
			name:      "no extension",
			pattern:   "{year}/{slug}",
			slug:      "post",
			timestamp: testTime,
			ext:       "",
			expected:  "2026/post",
		},
		{
			name:      "filename placeholder",
			pattern:   "posts/{filename}",
			slug:      "article",
			timestamp: time.Time{},
			ext:       ".md",
			expected:  "posts/article.md",
		},
		{
			name:      "date placeholders without timestamp",
			pattern:   "{year}/{month}/{slug}.json",
			slug:      "test",
			timestamp: time.Time{},
			ext:       "",
			expected:  "{year}/{month}/test.json",
		},
		{
			name:      "empty slug",
			pattern:   "{slug}.json",
			slug:      "",
			timestamp: time.Time{},
			ext:       "",
			wantErr:   true,
		},
		{
			name:      "complex pattern with all placeholders",
			pattern:   "content/{year}/{month}/{day}/{slug}{ext}",
			slug:      "my-entry",
			timestamp: testTime,
			ext:       ".json",
			expected:  "content/2026/01/15/my-entry.json",
		},
		{
			name:      "trailing slash in pattern",
			pattern:   "{year}/{month}//{slug}.json",
			slug:      "post",
			timestamp: testTime,
			ext:       "",
			expected:  "2026/01/post.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := NewPathPattern(tt.pattern)
			result, err := pattern.Generate(tt.slug, tt.timestamp, tt.ext)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestDefaultContentPattern(t *testing.T) {
	pattern := DefaultContentPattern()
	result, err := pattern.Generate("my-post", time.Time{}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "my-post.json"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestDefaultMediaPattern(t *testing.T) {
	pattern := DefaultMediaPattern()
	testTime := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	result, err := pattern.Generate("photo", testTime, ".jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "2026/01/photo.jpg"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestPathPattern_EmptyExtension(t *testing.T) {
	pattern := NewPathPattern("{slug}{ext}")
	result, err := pattern.Generate("test", time.Time{}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "test"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestPathPattern_MonthPadding(t *testing.T) {
	pattern := NewPathPattern("{year}/{month}/{day}/{slug}.json")
	testTime := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	result, err := pattern.Generate("post", testTime, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "2026/03/05/post.json"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}
