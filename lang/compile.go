package lang

import (
	"log/slog"
	"os"
	"strings"

	"github.com/expr-lang/expr"
)

// scope tracks the definitions visible at a given nesting level.
type scope struct {
	defs   []*Definition
	params []*Value
}

// CompileExprs walks the AST and compiles all TypeExpr values
// using expr-lang. Each expression gets an environment built from
// its position in the AST.
func (ast *AST) CompileExprs() error {
	if !ast.opts.compileExprs {
		return nil
	}

	processEnv := buildProcessEnvMap(ast.opts.processEnv)

	topScope := scope{
		defs:   ast.Definitions,
		params: nil,
	}

	for _, def := range ast.Definitions {
		err := compileDefinition(
			def, processEnv, []scope{topScope},
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// compileDefinition compiles all expr values within a definition.
// scopes provides the chain of visible scopes from outermost to
// innermost.
func compileDefinition(
	def *Definition,
	processEnv map[string]string,
	scopes []scope,
) error {
	if def.Value == nil {
		return nil
	}

	defScope := scope{
		defs:   nil,
		params: def.Parameters,
	}
	innerScopes := append(scopes, defScope)

	return compileValue(def.Value, processEnv, innerScopes)
}

// compileValue dispatches compilation based on value type.
func compileValue(
	v *Value,
	processEnv map[string]string,
	scopes []scope,
) error {
	switch v.Type {
	case TypeExpr:
		return compileExpr(v, processEnv, scopes)

	case TypeTuple:
		return compileTuple(v, processEnv, scopes)

	case TypeDefinition:
		if v.Definition == nil {
			return nil
		}

		return compileDefinition(v.Definition, processEnv, scopes)

	case TypeIdentifier, TypeBoolean, TypeNumber, TypeString:
		return nil

	default:
		return nil
	}
}

// compileTuple compiles all values within a tuple.
func compileTuple(
	v *Value,
	processEnv map[string]string,
	scopes []scope,
) error {
	if v.Tuple == nil {
		return nil
	}

	var tupleDefs []*Definition

	for _, val := range v.Tuple.Values {
		if val.Type == TypeDefinition && val.Definition != nil {
			tupleDefs = append(tupleDefs, val.Definition)
		}
	}

	tupleScope := scope{
		defs:   tupleDefs,
		params: nil,
	}
	innerScopes := append(scopes, tupleScope)

	for _, val := range v.Tuple.Values {
		if val.Type == TypeDefinition && val.Definition != nil {
			err := compileDefinition(
				val.Definition, processEnv, innerScopes,
			)
			if err != nil {
				return err
			}
		} else {
			err := compileValue(val, processEnv, innerScopes)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// compileExpr compiles a single expr-lang expression and stores the
// resulting program on the Value.
func compileExpr(
	v *Value,
	processEnv map[string]string,
	scopes []scope,
) error {
	source := v.ExprSource()

	// Skip empty expressions — expr-lang cannot compile them
	if source == "" {
		return nil
	}

	env := buildExprEnv(scopes, processEnv)

	program, err := expr.Compile(source, expr.Env(env))
	if err != nil {
		return ErrExprCompile.Wrap(err).
			With(slog.String("source", source))
	}

	v.Program = program

	return nil
}

// buildExprEnv constructs the environment map for expr compilation.
// It walks the scope chain from innermost to outermost, collecting
// all visible definitions and parameters.
func buildExprEnv(
	scopes []scope,
	processEnv map[string]string,
) map[string]any {
	env := make(map[string]any)

	for i := len(scopes) - 1; i >= 0; i-- {
		s := scopes[i]

		for _, def := range s.defs {
			name := def.Identifier.LiteralString()
			if _, exists := env[name]; !exists {
				env[name] = inferTypeExemplar(def.Value)
			}
		}

		for _, param := range s.params {
			if param.Token != nil {
				pName := param.Token.LiteralString()
				if _, exists := env[pName]; !exists {
					env[pName] = any(nil)
				}
			}
		}
	}

	env["env"] = envFunc(processEnv)

	return env
}

// inferTypeExemplar maps an aenv Value type to a Go type exemplar
// for expr-lang compilation.
func inferTypeExemplar(v *Value) any {
	if v == nil {
		return any(nil)
	}

	switch v.Type {
	case TypeBoolean:
		return false
	case TypeNumber:
		return inferNumberExemplar(v)
	case TypeString:
		return ""
	case TypeIdentifier, TypeExpr:
		return any(nil)
	case TypeTuple:
		return inferTupleExemplar(v)
	case TypeDefinition:
		return inferDefinitionExemplar(v)
	default:
		return any(nil)
	}
}

// inferNumberExemplar returns int64 or float64 based on the literal.
func inferNumberExemplar(v *Value) any {
	if v.Token != nil {
		lit := v.Token.LiteralString()
		if strings.ContainsAny(lit, ".eE") {
			return float64(0)
		}
	}

	return int64(0)
}

// inferTupleExemplar returns map[string]any for all-definition
// tuples, []any otherwise.
func inferTupleExemplar(v *Value) any {
	if v.Tuple == nil {
		return any(nil)
	}

	allDefs := true

	for _, val := range v.Tuple.Values {
		if val.Type != TypeDefinition {
			allDefs = false

			break
		}
	}

	if allDefs && len(v.Tuple.Values) > 0 {
		return map[string]any{}
	}

	return []any{}
}

// inferDefinitionExemplar returns a function exemplar for
// parametric definitions or recurses for the value.
func inferDefinitionExemplar(v *Value) any {
	if v.Definition == nil || v.Definition.Value == nil {
		return any(nil)
	}

	if len(v.Definition.Parameters) > 0 {
		return func(...any) any { return nil }
	}

	return inferTypeExemplar(v.Definition.Value)
}

// buildProcessEnvMap converts a "KEY=VALUE" string slice to a map.
// If envList is nil, os.Environ() is used.
func buildProcessEnvMap(envList []string, keyVal ...string) map[string]string {
	envList = append(envList, keyVal...)
	if len(envList) == 0 {
		envList = os.Environ()
	}

	result := make(map[string]string, len(envList))

	for _, entry := range envList {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			result[key] = value
		}
	}

	return result
}

// envFunc returns the built-in env() function that provides
// process environment access to expr programs.
func envFunc(processEnv map[string]string) func(string) string {
	return func(key string) string {
		return processEnv[key]
	}
}
