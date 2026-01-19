package lang

import (
	"encoding/json"
	"maps"
	"strconv"
	"strings"
)

// MarshalJSON implements json.Marshaler for AST.
func (ast *AST) MarshalJSON() ([]byte, error) {
	return json.Marshal(ast.ToMap())
}

// ToMap converts the AST to a native Go map structure.
func (ast *AST) ToMap() map[string]any {
	// Convert to a map where each definition name is a key
	result := make(map[string]any)

	for _, def := range ast.Definitions {
		name := def.Identifier.LiteralString()

		// If there are parameters, add them alongside the value
		if len(def.Parameters) > 0 {
			params := make([]any, len(def.Parameters))
			for i, param := range def.Parameters {
				params[i] = param.toNative()
			}

			value := def.Value.toNative()

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
			result[name] = def.Value.toNative()
		}
	}

	return result
}

// toNative converts a Value to its native Go type.
func (v *Value) toNative() any {
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
		s := v.Token.LiteralString()
		// Trim the whitespace enclosed within "{{" and "}}"
		s = strings.TrimPrefix(s, "{{")
		s = strings.TrimSuffix(s, "}}")

		s = strings.TrimSpace(s)
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

		// Check if all elements are Definitions - if so, return as an object
		allDefs := true

		for _, val := range v.Tuple.Aggregate {
			if val.Type != TypeDefinition {
				allDefs = false

				break
			}
		}

		if allDefs && len(v.Tuple.Aggregate) > 0 {
			// Return as object with definition names as keys
			result := make(map[string]any)

			for _, val := range v.Tuple.Aggregate {
				if val.Definition != nil {
					name := val.Definition.Identifier.LiteralString()
					// If definition has parameters, wrap with (parameters)
					if len(val.Definition.Parameters) > 0 {
						defData := make(map[string]any)

						params := make([]any, len(val.Definition.Parameters))
						for i, param := range val.Definition.Parameters {
							params[i] = param.toNative()
						}

						defData["(parameters)"] = params
						defData["(value)"] = val.Definition.Value.toNative()
						result[name] = defData
					} else {
						// No parameters: just use the value
						result[name] = val.Definition.Value.toNative()
					}
				}
			}

			return result
		}

		// Mixed tuple or all literals: return as array
		result := make([]any, 0, len(v.Tuple.Aggregate))
		for _, val := range v.Tuple.Aggregate {
			result = append(result, val.toNative())
		}

		return result

	case TypeDefinition:
		if v.Definition == nil {
			return nil
		}
		// Represent definition as a simple object {name: value}
		result := make(map[string]any)
		name := v.Definition.Identifier.LiteralString()

		// If definition has parameters, wrap with (parameters)
		if len(v.Definition.Parameters) > 0 {
			defData := make(map[string]any)

			params := make([]any, len(v.Definition.Parameters))
			for i, param := range v.Definition.Parameters {
				params[i] = param.toNative()
			}

			defData["(parameters)"] = params
			defData["(value)"] = v.Definition.Value.toNative()
			result[name] = defData
		} else {
			// No parameters: simple key-value
			result[name] = v.Definition.Value.toNative()
		}

		return result

	default:
		return nil
	}
}
