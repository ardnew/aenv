package lang

import (
	"reflect"
	"sort"
)

func sortedKeys[T any](m map[string]T) []string {
	if len(m) == 0 {
		return nil
	}

	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

func resultTypeName(value any) string {
	if value == nil {
		return "nil"
	}

	return reflect.TypeOf(value).String()
}
