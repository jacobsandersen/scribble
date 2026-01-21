package config

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

func ValidateAbsPath(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	return s != "" && path.IsAbs(s)
}

func ValidateLocalpath(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	return s != "" && filepath.IsLocal(s)
}

func ValidateIdentifier(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	if s == "" {
		return true
	}

	matched, err := regexp.MatchString(`^[A-Za-z_][A-Za-z0-9_]*$`, s)
	if err != nil {
		return false
	}

	return matched
}

// ValidatePathPattern validates path patterns used for file organization.
// It prevents path traversal attacks and ensures patterns are relative.
func ValidatePathPattern(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	if s == "" {
		return true // omitempty will handle this
	}

	// Check for path traversal (..) segments
	if strings.Contains(s, "..") {
		return false
	}

	// Check for absolute paths (Unix or Windows style)
	if path.IsAbs(s) || filepath.IsAbs(s) {
		return false
	}

	// Check for Windows drive letters (C:, D:, etc.)
	if len(s) >= 2 && s[1] == ':' {
		return false
	}

	// Check for null bytes or other dangerous characters
	if strings.ContainsAny(s, "\x00") {
		return false
	}

	return true
}
