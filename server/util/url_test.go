package util

import "testing"

func TestUrlIsInstance(t *testing.T) {
	tests := []struct {
		pattern string
		actual  string
		expect  bool
	}{
		{"https://example.com/{var}", "https://example.com/123", true},               // simple variable match
		{"https://example.com/a/{var}/b", "https://example.com/a/123/b", true},       // exact match with variable
		{"http://example.com/a/{var}/b", "https://example.com/a/123/c", false},       // scheme mismatch
		{"https://example.com/a/{var}/b", "https://example.com/a/123/c", false},      // path segment mismatch
		{"https://example.com/a/{var}/b", "https://example.com/a/123/b/c", false},    // extra path segment
		{"https://example.com/a/{var}/b", "https://sub.example.com/a/123/b", false},  // host mismatch
		{"https://example.com/{var1}/{var2}", "https://example.com/val1/val2", true}, // multiple variables
		{"https://example.com/{var1}/{var2}", "https://example.com/val1", false},     // missing variable
		{"https://example.com/{var}", "https://example.com/", false},                 // missing variable value
		{"https://example.com/{var}", "https://example.com", false},                  // missing path
	}

	for _, test := range tests {
		result := UrlIsInstance(test.pattern, test.actual)
		if result != test.expect {
			t.Errorf("UrlIsInstance(%q, %q) = %v; want %v", test.pattern, test.actual, result, test.expect)
		}
	}
}
