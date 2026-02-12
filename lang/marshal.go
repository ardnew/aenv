package lang

import (
	"encoding/json"
	"maps"
	"strconv"
)

// MarshalJSON implements json.Marshaler for AST.
func (ast *AST) MarshalJSON() ([]byte, error) {
	return json.Marshal(ast.ToMap())
}

// ToMap converts the AST to a native Go map structure.
func (ast *AST) ToMap() map[string]any {
	// Convert to a map where each namespace name is a key
	result := make(map[string]any)

	for _, ns := range ast.Namespaces {
		name := ns.Identifier.LiteralString()

		// If there are parameters, add them alongside the value
		if len(ns.Parameters) > 0 {
			params := make([]any, len(ns.Parameters))
			for i, param := range ns.Parameters {
				params[i] = param.ToNative()
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
	switch v.Type {
	case TypeIdentifier:
		return v.Token.LiteralString()

	case TypeNumber:
		// Parse the number to determine if it's an int or float
		s := v.Token.LiteralString()
		// Try parsing as int first
		if i, err := strconv.ParseInt(s, 0, 64); err == nil {
			return i
		}
		// Fall back to float
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
		// If both fail, return the string
		return s

	case TypeString:
		// Remove quotes from string literals
		s := v.Token.LiteralString()
		a := []rune(s)
		isEnclosed := func(r ...rune) bool {
			var lhs, rhs rune

			switch len(r) {
			case 0:
				return true
			case 1:
				lhs, rhs = r[0], r[0]
			default:
				lhs, rhs = r[0], r[1]
			}

			if len(a) < 2 {
				return false
			}

			return a[0] == lhs && a[len(a)-1] == rhs
		}

		if isEnclosed('"') || isEnclosed('`') || isEnclosed('\'') {
			// Unquote the string, handling escape sequencesgo
			if unquoted, err := strconv.Unquote(s); err == nil {
				return unquoted
			}
		}

		return s

	case TypeExpr:
		s := v.ExprSource()
		if len(s) > 0 {
			s = " " + s + " "
		}

		return "{{" + s + "}}"

	case TypeBoolean:
		s := v.Token.LiteralString()

		result, err := strconv.ParseBool(s)
		if err != nil {
			return false
		}

		return result

	case TypeTuple:
		if v.Tuple == nil {
			return nil
		}

		// Check if all elements are Namespaces - if so, return as an object
		allNamespaces := true

		for _, val := range v.Tuple.Values {
			if val.Type != TypeNamespace {
				allNamespaces = false

				break
			}
		}

		if allNamespaces && len(v.Tuple.Values) > 0 {
			// Return as object with namespace names as keys
			result := make(map[string]any)

			for _, val := range v.Tuple.Values {
				if val.Namespace != nil {
					name := val.Namespace.Identifier.LiteralString()
					// If namespace has parameters, wrap with (parameters)
					if len(val.Namespace.Parameters) > 0 {
						defData := make(map[string]any)

						params := make([]any, len(val.Namespace.Parameters))
						for i, param := range val.Namespace.Parameters {
							params[i] = param.ToNative()
						}

						defData["(parameters)"] = params
						defData["(value)"] = val.Namespace.Value.ToNative()
						result[name] = defData
					} else {
						// No parameters: just use the value
						result[name] = val.Namespace.Value.ToNative()
					}
				}
			}

			return result
		}

		// Mixed tuple or all literals: return as array
		result := make([]any, 0, len(v.Tuple.Values))
		for _, val := range v.Tuple.Values {
			result = append(result, val.ToNative())
		}

		return result

	case TypeNamespace:
		if v.Namespace == nil {
			return nil
		}
		// Represent namespace as a simple object {name: value}
		result := make(map[string]any)
		name := v.Namespace.Identifier.LiteralString()

		// If namespace has parameters, wrap with (parameters)
		if len(v.Namespace.Parameters) > 0 {
			defData := make(map[string]any)

			params := make([]any, len(v.Namespace.Parameters))
			for i, param := range v.Namespace.Parameters {
				params[i] = param.ToNative()
			}

			defData["(parameters)"] = params
			defData["(value)"] = v.Namespace.Value.ToNative()
			result[name] = defData
		} else {
			// No parameters: simple key-value
			result[name] = v.Namespace.Value.ToNative()
		}

		return result

	default:
		return nil
	}
}
