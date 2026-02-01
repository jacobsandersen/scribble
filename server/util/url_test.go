package util

import (
	"testing"
)

func TestUrlIsInstance(t *testing.T) {
	const patHttpsOneVar = "https://example.com/{var}"
	const patHttpsVarLitVar = "https://example.com/{var}/123/{var2}"
	const patHttpOneVar = "http://example.com/{var}"
	const instHttpsOneVar = "https://example.com/abc"
	const instHttpsVarLitVar = "https://example.com/abc/123/def"
	const instHttpsLitVarVar = "https://example.com/123/abc/def"
	const instDiffHostVarLitVar = "https://other.com/abc/123/def"

	tests := []struct {
		pattern string
		actual  string
		expect  bool
	}{
		{patHttpsOneVar, instHttpsOneVar, true},                   // simple variable match
		{patHttpsVarLitVar, instHttpsVarLitVar, true},             // exact match with variable
		{patHttpsVarLitVar, instHttpsLitVarVar, false},            // path segment mismatch
		{patHttpsVarLitVar, instHttpsLitVarVar + "/extra", false}, // extra path segment
		{patHttpOneVar, instHttpsOneVar, false},                   // scheme mismatch
		{patHttpsVarLitVar, instDiffHostVarLitVar, false},         // host mismatch
	}

	for _, test := range tests {
		result := UrlIsInstance(test.pattern, test.actual)
		if result != test.expect {
			t.Errorf("UrlIsInstance(%q, %q) = %v; want %v", test.pattern, test.actual, result, test.expect)
		}
	}
}
