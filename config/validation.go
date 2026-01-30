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

func ValidatePathPattern(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	if s == "" {
		return false
	}

	if strings.Contains(s, "..") {
		return false
	}

	if path.IsAbs(s) || filepath.IsAbs(s) {
		return false
	}

	if len(s) >= 2 && s[1] == ':' {
		return false
	}

	if strings.ContainsAny(s, "\x00") {
		return false
	}

	return true
}
