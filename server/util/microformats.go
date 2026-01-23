package util

import (
	"errors"
	"fmt"
)

type MicroformatProperties map[string][]any

type Mf2Document struct {
	Type       []string              `json:"type"`
	Properties MicroformatProperties `json:"properties"`
}

func ValidateMf2(doc Mf2Document) error {
	if len(doc.Type) == 0 {
		return errors.New("mf2 type array must not be empty")
	}

	for i, t := range doc.Type {
		if t == "" {
			return fmt.Errorf("mf2 type[%d] is empty", i)
		}
	}

	if doc.Properties == nil {
		return errors.New("mf2 properties must not be nil")
	}

	for key, values := range doc.Properties {
		if key == "" {
			return errors.New("mf2 property names must not be empty")
		}

		for i, v := range values {
			switch x := v.(type) {
			case string:
				// ok
			case map[string]any:
				// ok - embedded object (e.g., {html: ["..."], value: ["..."]})
			case Mf2Document:
				// recursively validate embedded mf2
				if err := ValidateMf2(x); err != nil {
					return fmt.Errorf("invalid embedded mf2 in property %q[%d]: %w", key, i, err)
				}
			default:
				return fmt.Errorf("mf2 property %q contains invalid value type %T at index %d", key, x, i)
			}
		}
	}

	return nil
}
