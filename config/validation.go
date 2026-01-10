package config

import (
	"path"

	"github.com/go-playground/validator/v10"
)

func ValidateAbsPath(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	return s != "" && path.IsAbs(s)
}
