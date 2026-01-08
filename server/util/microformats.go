package util

import (
	"errors"
	"fmt"
)

func ValidateMf2(doc map[string]any) error {
	rawType, ok := doc["type"]
	if !ok {
		return errors.New("missing mf2 type")
	}

	typeArr, ok := rawType.([]string)
	if !ok {
		return errors.New("mf2 type must be an array of strings")
	}

	if len(typeArr) == 0 {
		return errors.New("mf2 type array must not be empty")
	}

	rawProps, ok := doc["properties"]
	if !ok {
		return errors.New("missing mf2 properties")
	}

	props, ok := rawProps.(map[string]any)
	if !ok {
		return errors.New("mf2 properties must be an object")
	}

	for key, val := range props {
		if key == "" {
			return errors.New("mf2 property names must not be empty")
		}

		if _, ok := val.([]string); !ok {
			return fmt.Errorf("mf2 property %q must be []string, got %T", key, val)
		}
	}

	return nil
}
