package config

import (
	"path"
	"path/filepath"
	"regexp"

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
