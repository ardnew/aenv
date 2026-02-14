package lang

import (
	"encoding/json"
	"maps"
	"strconv"
)

// MarshalJSON implements json.Marshaler for AST.
func (a *AST) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.ToMap())
}

// ToMap converts the AST to a native Go map structure.
func (a *AST) ToMap() map[string]any {
	// Convert to a map where each namespace name is a key
	result := make(map[string]any)

	for _, ns := range a.Namespaces {
		name := ns.Name

		// If there are parameters, add them alongside the value
		if len(ns.Params) > 0 {
			params := make([]string, len(ns.Params))
			for i, param := range ns.Params {
				if param.Variadic {
					params[i] = "..." + param.Name
				} else {
					params[i] = param.Name
				}
			}

			value := ns.Value.ToNative()

			// If the value is a map, flatten it into the same object as (parameters)
			if valueMap, ok := value.(map[string]any); ok {
				defData := make(map[string]any)

				defData["(parameters)"] = params

				// Merge value map keys into defData
				maps.Copy(defData, valueMap)

				result[name] = defData
			} else {
				// Value is not a map (e.g., array or literal), so keep it under (value)
				defData := make(map[string]any)

				defData["(parameters)"] = params
				defData["(value)"] = value

				result[name] = defData
			}
		} else {
			// No parameters: output value directly
			result[name] = ns.Value.ToNative()
		}
	}

	return result
}

// ToNative converts a Value to its native Go type.
func (v *Value) ToNative() any {
	if v == nil {
		return nil
	}

	switch v.Kind {
	case KindExpr:
		// Try to parse as literal types first
		// Trim whitespace from expression source
		s := v.Source
		// Remove leading and trailing whitespace but preserve string content
		for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
			s = s[1:]
		}

		for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
			s = s[:len(s)-1]
		}

		// Check for boolean
		if s == "true" {
			return true
		}

		if s == "false" {
			return false
		}

		// Try parsing as number
		if i, err := strconv.ParseInt(s, 0, 64); err == nil {
			return i
		}

		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}

		// Check for quoted string
		if len(s) >= 2 {
			first, last := s[0], s[len(s)-1]
			if (first == '"' && last == '"') ||
				(first == '\'' && last == '\'') ||
				(first == '`' && last == '`') {
				if unquoted, err := strconv.Unquote(s); err == nil {
					return unquoted
				}
			}
		}

		// Return as-is (likely an identifier or expression)
		return s

	case KindBlock:
		// Block: convert to map
		if v.Entries == nil || len(v.Entries) == 0 {
			return make(map[string]any)
		}

		result := make(map[string]any)

		for _, ns := range v.Entries {
			name := ns.Name

			// If namespace has parameters, wrap with (parameters)
			if len(ns.Params) > 0 {
				defData := make(map[string]any)

				params := make([]string, len(ns.Params))
				for i, param := range ns.Params {
					if param.Variadic {
						params[i] = "..." + param.Name
					} else {
						params[i] = param.Name
					}
				}

				defData["(parameters)"] = params
				defData["(value)"] = ns.Value.ToNative()

				result[name] = defData
			} else {
				// No parameters: just use the value
				result[name] = ns.Value.ToNative()
			}
		}

		return result

	default:
		return nil
	}
}
