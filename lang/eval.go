package lang

import (
	"fmt"
	"log/slog"
	"maps"
	"strconv"
	"strings"

	"github.com/expr-lang/expr/vm"
)

// EvaluateDefinition evaluates a definition with the given parameter bindings.
// The args slice provides values for each parameter in order.
// Returns the fully evaluated result.
//
// Options are applied as overrides to ast.opts for this evaluation only,
// enabling thread-safe concurrent evaluations with different processEnv.
func (ast *AST) EvaluateDefinition(
	name string,
	args []string,
	opts ...Option,
) (any, error) {
	// Copy ast.opts locally for thread-safe concurrent evaluation
	localOpts := ast.opts
	for _, opt := range opts {
		// Apply option to a temporary AST to extract the setting
		var temp AST

		temp.opts = localOpts
		opt(&temp)
		localOpts = temp.opts
	}

	def, found := ast.GetDefinition(name)
	if !found {
		return nil, ErrDefinitionNotFound.
			With(slog.String("name", name))
	}

	if len(args) != len(def.Parameters) {
		return nil, ErrParamCountMismatch.
			With(
				slog.String("name", name),
				slog.Int("expected", len(def.Parameters)),
				slog.Int("got", len(args)),
			)
	}

	processEnv := buildProcessEnvMap(localOpts.processEnv)

	// Build parameter bindings map
	params := make(map[string]any, len(args))

	for i, param := range def.Parameters {
		if param.Token != nil {
			paramName := param.Token.LiteralString()
			params[paramName] = parseArgValue(args[i])
		}
	}

	// Build evaluation context from AST scope
	ctx := &evalContext{
		ast:        ast,
		processEnv: processEnv,
		params:     params,
	}

	return ctx.evaluateValue(def.Value)
}

// evalContext holds the state for recursive evaluation.
type evalContext struct {
	ast        *AST
	processEnv map[string]string
	params     map[string]any
}

// evaluateValue recursively evaluates a Value to its Go representation.
func (ctx *evalContext) evaluateValue(v *Value) (any, error) {
	if v == nil {
		return nil, nil
	}

	switch v.Type {
	case TypeBoolean:
		return ctx.evaluateBoolean(v)

	case TypeNumber:
		return ctx.evaluateNumber(v)

	case TypeString:
		return ctx.evaluateString(v)

	case TypeIdentifier:
		return ctx.evaluateIdentifier(v)

	case TypeExpr:
		return ctx.evaluateExpr(v)

	case TypeTuple:
		return ctx.evaluateTuple(v)

	case TypeDefinition:
		return ctx.evaluateDefinition(v)

	default:
		return nil, ErrInvalidValueType.
			With(slog.String("type", v.Type.String()))
	}
}

// evaluateBoolean evaluates a boolean literal.
func (ctx *evalContext) evaluateBoolean(v *Value) (bool, error) {
	if v.Token == nil {
		return false, nil
	}

	s := v.Token.LiteralString()

	result, err := strconv.ParseBool(s)
	if err != nil {
		return false, ErrInvalidBoolean.Wrap(err).
			With(slog.String("value", s))
	}

	return result, nil
}

// evaluateNumber evaluates a number literal.
func (ctx *evalContext) evaluateNumber(v *Value) (any, error) {
	if v.Token == nil {
		return int64(0), nil
	}

	s := v.Token.LiteralString()

	// Try int first
	if i, err := strconv.ParseInt(s, 0, 64); err == nil {
		return i, nil
	}

	// Fall back to float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, nil
	}

	return nil, ErrInvalidNumber.
		With(slog.String("value", s))
}

// evaluateString evaluates a string literal.
func (ctx *evalContext) evaluateString(v *Value) (string, error) {
	if v.Token == nil {
		return "", nil
	}

	s := v.Token.LiteralString()

	// Remove surrounding quotes if present
	if len(s) >= 2 {
		first, last := s[0], s[len(s)-1]
		if (first == '"' && last == '"') ||
			(first == '\'' && last == '\'') ||
			(first == '`' && last == '`') {
			if unquoted, err := strconv.Unquote(s); err == nil {
				return unquoted, nil
			}
		}
	}

	return s, nil
}

// evaluateIdentifier resolves an identifier reference.
func (ctx *evalContext) evaluateIdentifier(v *Value) (any, error) {
	if v.Token == nil {
		return nil, nil
	}

	name := v.Token.LiteralString()

	// Check parameters first
	if val, ok := ctx.params[name]; ok {
		return val, nil
	}

	// Check AST definitions
	def, found := ctx.ast.GetDefinition(name)
	if !found {
		return nil, ErrDefinitionNotFound.
			With(slog.String("name", name))
	}

	// Evaluate the referenced definition's value
	return ctx.evaluateValue(def.Value)
}

// evaluateExpr runs a compiled expr program.
func (ctx *evalContext) evaluateExpr(v *Value) (any, error) {
	if v.Program == nil {
		// Expression not compiled - return source as-is
		return v.ExprSource(), nil
	}

	// Build runtime environment
	env := ctx.buildRuntimeEnv()

	result, err := vm.Run(v.Program, env)
	if err != nil {
		return nil, ErrExprEvaluate.Wrap(err).
			With(slog.String("source", v.ExprSource()))
	}

	return result, nil
}

// evaluateTuple evaluates a tuple of values.
func (ctx *evalContext) evaluateTuple(v *Value) (any, error) {
	if v.Tuple == nil {
		return nil, nil
	}

	// Check if all elements are definitions
	allDefs := true

	for _, val := range v.Tuple.Values {
		if val.Type != TypeDefinition {
			allDefs = false

			break
		}
	}

	if allDefs && len(v.Tuple.Values) > 0 {
		// First pass: build a map of simple values for the scope
		// This allows expressions to reference sibling definitions
		tupleScope := make(map[string]any)

		for _, val := range v.Tuple.Values {
			if val.Definition != nil {
				name := val.Definition.Identifier.LiteralString()
				// Only include non-expression values in the scope
				// to avoid circular evaluation
				if val.Definition.Value != nil &&
					val.Definition.Value.Type != TypeExpr {
					tupleScope[name] = ctx.resolveForEnv(val.Definition.Value)
				}
			}
		}

		// Create a child context with the tuple scope
		childCtx := &evalContext{
			ast:        ctx.ast,
			processEnv: ctx.processEnv,
			params:     make(map[string]any),
		}

		// Copy parent params
		maps.Copy(childCtx.params, ctx.params)

		// Add tuple scope to child params
		maps.Copy(childCtx.params, tupleScope)

		// Second pass: evaluate all definitions with the enriched context
		result := make(map[string]any)

		for _, val := range v.Tuple.Values {
			if val.Definition != nil {
				name := val.Definition.Identifier.LiteralString()

				evaluated, err := childCtx.evaluateValue(val.Definition.Value)
				if err != nil {
					return nil, err
				}

				result[name] = evaluated
			}
		}

		return result, nil
	}

	// Mixed tuple or all literals: return as slice
	result := make([]any, 0, len(v.Tuple.Values))

	for _, val := range v.Tuple.Values {
		evaluated, err := ctx.evaluateValue(val)
		if err != nil {
			return nil, err
		}

		result = append(result, evaluated)
	}

	return result, nil
}

// evaluateDefinition evaluates a nested definition.
func (ctx *evalContext) evaluateDefinition(v *Value) (any, error) {
	if v.Definition == nil {
		return nil, nil
	}

	return ctx.evaluateValue(v.Definition.Value)
}

// buildRuntimeEnv constructs the environment for expr execution.
func (ctx *evalContext) buildRuntimeEnv() map[string]any {
	env := make(map[string]any)

	// Add parameters first (they take precedence)
	maps.Copy(env, ctx.params)

	// Add top-level definitions
	// For simple values, we can evaluate them directly
	// For expr values, we provide nil to avoid infinite recursion
	// (the expr references will be resolved when the program runs)
	for _, def := range ctx.ast.Definitions {
		name := def.Identifier.LiteralString()
		if _, exists := env[name]; !exists {
			env[name] = ctx.resolveForEnv(def.Value)
		}
	}

	// Add env function
	env["env"] = envFunc(ctx.processEnv)

	return env
}

// resolveForEnv converts a Value to a Go value suitable for the expr
// environment.
// This avoids infinite recursion by not evaluating expressions.
func (ctx *evalContext) resolveForEnv(v *Value) any {
	if v == nil {
		return nil
	}

	switch v.Type {
	case TypeBoolean:
		if v.Token == nil {
			return false
		}

		result, _ := strconv.ParseBool(v.Token.LiteralString())

		return result

	case TypeNumber:
		if v.Token == nil {
			return int64(0)
		}

		s := v.Token.LiteralString()
		if i, err := strconv.ParseInt(s, 0, 64); err == nil {
			return i
		}

		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}

		return int64(0)

	case TypeString:
		if v.Token == nil {
			return ""
		}

		s := v.Token.LiteralString()
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

		return s

	case TypeIdentifier:
		// Look up the identifier
		if v.Token == nil {
			return nil
		}

		name := v.Token.LiteralString()

		// Check parameters first
		if val, ok := ctx.params[name]; ok {
			return val
		}

		// Check definitions (but don't recurse into exprs)
		if def, ok := ctx.ast.GetDefinition(name); ok {
			return ctx.resolveForEnv(def.Value)
		}

		return nil

	case TypeExpr:
		// Cannot evaluate expr during env building - return nil
		return nil

	case TypeTuple:
		if v.Tuple == nil {
			return nil
		}

		// Check if all elements are definitions
		allDefs := true

		for _, val := range v.Tuple.Values {
			if val.Type != TypeDefinition {
				allDefs = false

				break
			}
		}

		if allDefs && len(v.Tuple.Values) > 0 {
			result := make(map[string]any)

			for _, val := range v.Tuple.Values {
				if val.Definition != nil {
					name := val.Definition.Identifier.LiteralString()
					result[name] = ctx.resolveForEnv(val.Definition.Value)
				}
			}

			return result
		}

		result := make([]any, 0, len(v.Tuple.Values))

		for _, val := range v.Tuple.Values {
			result = append(result, ctx.resolveForEnv(val))
		}

		return result

	case TypeDefinition:
		if v.Definition == nil {
			return nil
		}

		return ctx.resolveForEnv(v.Definition.Value)

	default:
		return nil
	}
}

// parseArgValue attempts to parse a string argument into an appropriate type.
func parseArgValue(s string) any {
	// Try boolean
	if b, err := strconv.ParseBool(s); err == nil {
		return b
	}

	// Try integer
	if i, err := strconv.ParseInt(s, 0, 64); err == nil {
		return i
	}

	// Try float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}

	// Default to string
	return s
}

// FormatResult formats an evaluation result for output.
// This produces a native aenv syntax representation.
func FormatResult(result any) string {
	return formatResultValue(result)
}

// formatResultValue recursively formats a Go value as aenv syntax.
func formatResultValue(v any) string {
	switch val := v.(type) {
	case nil:
		return ""

	case bool:
		return strconv.FormatBool(val)

	case int:
		return strconv.Itoa(val)

	case int64:
		return strconv.FormatInt(val, 10)

	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)

	case string:
		// Quote strings that contain special characters
		if needsQuoting(val) {
			return strconv.Quote(val)
		}

		return val

	case []any:
		return formatSlice(val)

	case map[string]any:
		return formatMap(val)

	default:
		return fmt.Sprintf("%v", val)
	}
}

// needsQuoting returns true if a string needs to be quoted.
func needsQuoting(s string) bool {
	if s == "" {
		return true
	}

	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' ||
			r == '"' || r == '\'' || r == '\\' ||
			r == '{' || r == '}' || r == ':' || r == ',' {
			return true
		}
	}

	return false
}

// formatSlice formats a slice as aenv tuple syntax.
func formatSlice(vals []any) string {
	if len(vals) == 0 {
		return "{}"
	}

	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = formatResultValue(v)
	}

	return "{ " + strings.Join(parts, ", ") + " }"
}

// formatMap formats a map as aenv tuple with definitions.
func formatMap(m map[string]any) string {
	if len(m) == 0 {
		return "{}"
	}

	parts := make([]string, 0, len(m))

	for k, v := range m {
		parts = append(parts, k+" : "+formatResultValue(v))
	}

	return "{ " + strings.Join(parts, ", ") + " }"
}
