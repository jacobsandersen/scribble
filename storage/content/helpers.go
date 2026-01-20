package content

import "reflect"

// deleteValues removes elements present in toRemove from values using deep equality.
func deleteValues(values []any, toRemove []any) []any {
	if len(values) == 0 || len(toRemove) == 0 {
		return values
	}

	var remaining []any
	for _, v := range values {
		if !containsValue(toRemove, v) {
			remaining = append(remaining, v)
		}
	}

	return remaining
}

func containsValue(list []any, value any) bool {
	for _, candidate := range list {
		if reflect.DeepEqual(candidate, value) {
			return true
		}
	}

	return false
}
