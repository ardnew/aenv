package repl

import (
	"reflect"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/expr-lang/expr/builtin"

	"github.com/ardnew/aenv/lang"
)

// Type-name strings used in signature parameter display.
const (
	typeNameInt    = "int"
	typeNameFloat  = "float"
	typeNameString = "string"
	typeNameArg    = "arg"
	typeNameArray  = "array"
)

// signatureCache stores precomputed function signatures to avoid repeated
// reflection and string building on every keystroke.
type signatureCache struct {
	signature string
	params    []string
}

var (
	// exprLangCache caches signatures for expr-lang builtin functions.
	exprLangCache     map[string]signatureCache
	exprLangCacheOnce sync.Once

	// projectBuiltinCache caches signatures for project builtin functions.
	projectBuiltinCache     map[string]signatureCache
	projectBuiltinCacheOnce sync.Once
)

// initExprLangCache initializes the expr-lang builtin signature cache.
func initExprLangCache() {
	exprLangCache = make(map[string]signatureCache, len(builtin.Builtins))

	for _, fn := range builtin.Builtins {
		sig, params, ok := getExprLangBuiltinSignatureUncached(fn.Name)
		if ok {
			exprLangCache[fn.Name] = signatureCache{
				signature: sig,
				params:    params,
			}
		}
	}
}

// initProjectBuiltinCache initializes the project builtin signature cache.
func initProjectBuiltinCache() {
	projectBuiltinCache = make(map[string]signatureCache)

	// Cache all top-level builtin environment keys
	env := lang.BuiltinEnvCache()

	for key := range env {
		// Try common nested paths for each top-level key
		if m, ok := env[key].(map[string]any); ok {
			for subkey := range m {
				fullName := key + "." + subkey

				sig, params, ok := getBuiltinSignatureUncached(fullName)
				if ok {
					projectBuiltinCache[fullName] = signatureCache{
						signature: sig,
						params:    params,
					}
				}
			}
		}

		// Also cache top-level if it's a function
		sig, params, ok := getBuiltinSignatureUncached(key)
		if ok {
			projectBuiltinCache[key] = signatureCache{
				signature: sig,
				params:    params,
			}
		}
	}
}

// getExprLangBuiltinSignature retrieves a cached signature for an expr-lang
// builtin function. On first call, initializes the cache.
func getExprLangBuiltinSignature(funcName string) (string, []string, bool) {
	exprLangCacheOnce.Do(initExprLangCache)

	cached, ok := exprLangCache[funcName]
	if !ok {
		return "", nil, false
	}

	return cached.signature, cached.params, true
}

// getExprLangBuiltinSignatureUncached extracts the signature for an expr-lang
// builtin function using reflection. This is the uncached implementation used
// during cache initialization.
func getExprLangBuiltinSignatureUncached(
	funcName string,
) (string, []string, bool) {
	// Look up the function in expr-lang's builtin index
	idx, ok := builtin.Index[funcName]
	if !ok {
		return "", nil, false
	}

	fn := builtin.Builtins[idx]

	// Try to extract parameter information from Types field first (most accurate)
	if len(fn.Types) > 0 {
		// Use the first type signature (most common case)
		firstType := fn.Types[0]
		if firstType.Kind() == reflect.Func {
			return extractSignatureFromFuncType(funcName, firstType)
		}
	}

	// Fallback: extract from Fast/Func/Safe functions
	var (
		params        []string
		funcToInspect any
	)

	switch {
	case fn.Fast != nil:
		funcToInspect = fn.Fast
	case fn.Func != nil:
		funcToInspect = fn.Func
	case fn.Safe != nil:
		funcToInspect = fn.Safe
	default:
		// No function to inspect - use special case or generic parameter
		params = []string{getGenericParamName(funcName, 0)}

		goto buildSignature
	}

	// Use reflection on the function to extract parameter types
	{
		t := reflect.TypeOf(funcToInspect)
		if t.Kind() == reflect.Func {
			numParams := t.NumIn()
			isVariadic := t.IsVariadic()

			for i := range numParams {
				paramType := t.In(i)

				var name string

				if isVariadic && i == numParams-1 {
					// Last parameter of variadic function
					elemType := paramType.Elem()
					name = "..." + formatSemanticTypeName(funcName, i, elemType)
				} else {
					name = formatSemanticTypeName(funcName, i, paramType)
				}

				params = append(params, name)
			}
		}
	}

buildSignature:
	// Build signature string
	var sig strings.Builder

	sig.WriteString(funcName)
	sig.WriteString("(")

	for i, param := range params {
		if i > 0 {
			sig.WriteString(", ")
		}

		sig.WriteString(param)
	}

	sig.WriteString(")")

	return sig.String(), params, true
}

// getGenericParamName returns a semantic parameter name for functions that
// operate on generic types.
func getGenericParamName(funcName string, _ int) string {
	// Special cases for well-known functions
	switch funcName {
	case "len", "type", typeNameInt, typeNameFloat, typeNameString:
		return "v"
	default:
		return typeNameArg
	}
}

// extractSignatureFromFuncType extracts a function signature from a
// reflect.Type
// representing a function. It provides semantic parameter names based on the
// parameter types.
func extractSignatureFromFuncType(
	funcName string,
	funcType reflect.Type,
) (string, []string, bool) {
	if funcType.Kind() != reflect.Func {
		return "", nil, false
	}

	numParams := funcType.NumIn()
	isVariadic := funcType.IsVariadic()
	params := make([]string, 0, numParams)

	for i := range numParams {
		paramType := funcType.In(i)

		var name string

		if isVariadic && i == numParams-1 {
			// Last parameter of variadic function
			elemType := paramType.Elem()
			name = "..." + formatSemanticTypeName(funcName, i, elemType)
		} else {
			name = formatSemanticTypeName(funcName, i, paramType)
		}

		params = append(params, name)
	}

	// Build signature string
	var sig strings.Builder
	sig.WriteString(funcName)
	sig.WriteString("(")

	for i, param := range params {
		if i > 0 {
			sig.WriteString(", ")
		}

		sig.WriteString(param)
	}

	sig.WriteString(")")

	return sig.String(), params, true
}

// formatSemanticTypeName converts a reflect.Type to a semantic parameter name.
// This provides better names than formatTypeName by considering the type's
// structure and the function context.
func formatSemanticTypeName(
	funcName string,
	paramIdx int,
	t reflect.Type,
) string {
	// Special cases for common expr-lang builtin patterns
	switch funcName {
	case "join":
		if paramIdx == 0 {
			return typeNameArray
		}

		if paramIdx == 1 && t.Kind() == reflect.String {
			return "separator"
		}
	case "split":
		if paramIdx == 0 {
			return typeNameString
		}

		if paramIdx == 1 && t.Kind() == reflect.String {
			return "separator"
		}
	}

	// General type-based naming
	switch t.Kind() {
	case reflect.Func:
		// Check if it's a predicate (returns bool)
		if t.NumOut() > 0 && t.Out(0).Kind() == reflect.Bool {
			return "predicate"
		}

		return "func"
	case reflect.String:
		return typeNameString
	case reflect.Slice:
		return typeNameArray
	case reflect.Interface:
		// Generic interface{} - use context-aware name
		return "v"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return typeNameInt
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		return "uint"
	case reflect.Float32, reflect.Float64:
		return typeNameFloat
	case reflect.Bool:
		return "bool"
	case reflect.Map:
		return "map"
	case reflect.Ptr:
		return formatSemanticTypeName(funcName, paramIdx, t.Elem())
	default:
		if t.Name() != "" {
			return t.Name()
		}

		return typeNameArg
	}
}

// ExprLangBuiltinNames returns the names of all expr-lang builtin functions.
func ExprLangBuiltinNames() []string {
	names := make([]string, 0, len(builtin.Builtins))
	for _, fn := range builtin.Builtins {
		names = append(names, fn.Name)
	}

	return names
}

// signatureHintStyle styles for parameter hints.
var (
	signatureStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	signatureNameStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("6")).
				Bold(true)
	currentParamStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("11")).
				Bold(true)
	signatureSeparatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// functionCall represents a detected function call in the input.
type functionCall struct {
	name     string // fully qualified function name (e.g., "path.cat")
	argIndex int    // current argument index (0-based)
	inCall   bool   // true if cursor is inside parameter list
}

// detectFunctionCall analyzes the input to determine if the cursor is inside
// a function call's parameter list. It returns the function name, current
// argument index, and whether we're inside a call.
func detectFunctionCall(input string, cursor int) functionCall {
	if cursor > len(input) {
		cursor = len(input)
	}

	// Scan backward from cursor to find the opening paren of a function call.
	// Track nested parens so we find the correct one.
	parenDepth := 0
	openParenPos := -1

	for i := cursor - 1; i >= 0; i-- {
		ch, size := utf8.DecodeLastRuneInString(input[:i+1])

		switch ch {
		case ')':
			parenDepth++
		case '(':
			if parenDepth == 0 {
				openParenPos = i

				goto foundOpenParen
			}

			parenDepth--
		}

		// Move to start of this rune
		if i > 0 {
			i -= (size - 1)
		}
	}

foundOpenParen:
	if openParenPos == -1 {
		return functionCall{inCall: false}
	}

	// Extract function name before the '('
	// Walk backward collecting identifier characters and dots
	nameEnd := openParenPos
	nameStart := openParenPos

	for nameStart > 0 {
		r, size := utf8.DecodeLastRuneInString(input[:nameStart])

		// Function names can include dots (for namespaces), letters, numbers, and
		// hyphens
		if r == '.' || r == '_' || r == '-' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			nameStart -= size
		} else {
			break
		}
	}

	funcName := strings.TrimSpace(input[nameStart:nameEnd])
	if funcName == "" {
		return functionCall{inCall: false}
	}

	// Count arguments by counting commas at depth 0 in the parameter list
	argIndex := 0
	depth := 0

	for i := openParenPos + 1; i < cursor; i++ {
		ch, size := utf8.DecodeRuneInString(input[i:])

		switch ch {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				argIndex++
			}
		}

		i += size - 1
	}

	return functionCall{
		name:     funcName,
		argIndex: argIndex,
		inCall:   true,
	}
}

// getSignature retrieves the function signature for a given function name.
// It looks in both user-defined namespaces (AST) and built-in functions.
// Returns empty string if the function is not found.
func getSignature(
	ast *lang.AST,
	funcName string,
) (signature string, params []string) {
	// Try to resolve as a namespace in the AST first
	segments := strings.Split(funcName, ".")

	if len(segments) == 1 {
		// Top-level namespace
		if ns, ok := ast.GetNamespace(funcName); ok {
			return formatSignature(ns.Name, ns.Params), extractParamNames(ns.Params)
		}
	} else if len(segments) > 1 {
		// Nested namespace - walk the path
		ns, ok := ast.GetNamespace(segments[0])
		if ok {
			val := ns.Value

			// Walk through remaining segments
			for _, seg := range segments[1 : len(segments)-1] {
				val = findChildValue(val, seg)
				if val == nil {
					break
				}
			}

			if val != nil && val.Kind == lang.KindBlock {
				// Look for the final segment as a namespace in this block
				finalName := segments[len(segments)-1]
				for _, entry := range val.Entries {
					if entry.Name == finalName {
						return formatSignature(funcName, entry.Params), extractParamNames(entry.Params)
					}
				}
			}
		}
	}

	// Check expr-lang builtin functions using reflection
	if sig, params, ok := getExprLangBuiltinSignature(funcName); ok {
		return sig, params
	}

	// Check project built-in functions using reflection
	if sig, params, ok := getBuiltinSignature(funcName); ok {
		return sig, params
	}

	return "", nil
}

// getBuiltinSignature retrieves a cached signature for a project builtin
// function. On first call, initializes the cache.
func getBuiltinSignature(funcName string) (string, []string, bool) {
	projectBuiltinCacheOnce.Do(initProjectBuiltinCache)

	cached, ok := projectBuiltinCache[funcName]
	if !ok {
		// Not in cache - could be a user-defined namespace that changed
		// Fall back to uncached lookup
		return getBuiltinSignatureUncached(funcName)
	}

	return cached.signature, cached.params, true
}

// getBuiltinSignatureUncached uses reflection to extract the signature of a
// builtin function from the lang environment cache. Returns (signature, params,
// true) if found, ("", nil, false) otherwise. This is the uncached
// implementation used during cache initialization.
func getBuiltinSignatureUncached(funcName string) (string, []string, bool) {
	// Get the builtin environment
	env := lang.BuiltinEnvCache()

	// Split the function name to handle nested maps (e.g., "path.cat")
	segments := strings.Split(funcName, ".")

	var current any = env

	// Navigate through nested maps
	for _, seg := range segments {
		m, ok := current.(map[string]any)
		if !ok {
			return "", nil, false
		}

		val, exists := m[seg]
		if !exists {
			return "", nil, false
		}

		current = val
	}

	// Current should now be a function; use reflection to inspect it
	t := reflect.TypeOf(current)
	if t == nil || t.Kind() != reflect.Func {
		return "", nil, false
	}

	// Build parameter list from type information
	numParams := t.NumIn()
	isVariadic := t.IsVariadic()
	params := make([]string, 0, numParams)

	for i := range numParams {
		paramType := t.In(i)

		var name string

		if isVariadic && i == numParams-1 {
			// Last parameter of variadic function - extract element type
			elemType := paramType.Elem()
			name = "..." + formatTypeName(elemType)
		} else {
			name = formatTypeName(paramType)
		}

		params = append(params, name)
	}

	// Build signature string
	var sig strings.Builder
	sig.WriteString(funcName)
	sig.WriteString("(")

	for i, param := range params {
		if i > 0 {
			sig.WriteString(", ")
		}

		sig.WriteString(param)
	}

	sig.WriteString(")")

	return sig.String(), params, true
}

// formatTypeName converts a reflect.Type to a readable parameter name.
// Examples: "string", "int", "bool", "func".
func formatTypeName(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Func:
		return "func"
	case reflect.String:
		return typeNameString
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return typeNameInt
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		return "uint"
	case reflect.Float32, reflect.Float64:
		return typeNameFloat
	case reflect.Bool:
		return "bool"
	case reflect.Slice:
		return "slice"
	case reflect.Map:
		return "map"
	case reflect.Ptr:
		return formatTypeName(t.Elem())
	default:
		// Fallback to the type's name if available
		if t.Name() != "" {
			return t.Name()
		}

		return typeNameArg
	}
}

// findChildValue looks up a child namespace by name within a block value.
func findChildValue(v *lang.Value, name string) *lang.Value {
	if v == nil || v.Kind != lang.KindBlock {
		return nil
	}

	for _, child := range v.Entries {
		if child.Name == name {
			return child.Value
		}
	}

	return nil
}

// formatSignature formats a function signature with parameter names.
func formatSignature(name string, params []lang.Param) string {
	if len(params) == 0 {
		return name + "()"
	}

	paramNames := make([]string, len(params))
	for i, p := range params {
		if p.Variadic {
			paramNames[i] = "..." + p.Name
		} else {
			paramNames[i] = p.Name
		}
	}

	return name + "(" + strings.Join(paramNames, ", ") + ")"
}

// extractParamNames extracts parameter names from a parameter list.
func extractParamNames(params []lang.Param) []string {
	names := make([]string, len(params))
	for i, p := range params {
		if p.Variadic {
			names[i] = "..." + p.Name
		} else {
			names[i] = p.Name
		}
	}

	return names
}

// renderSignatureHint renders the function signature with the current
// parameter highlighted.
func renderSignatureHint(
	signature string,
	params []string,
	currentArgIdx int,
) string {
	if signature == "" {
		return ""
	}

	// Parse signature: "funcName(param1, param2, ...)"
	openParen := strings.Index(signature, "(")
	if openParen == -1 {
		return signatureStyle.Render(signature)
	}

	funcName := signature[:openParen]

	closeParen := strings.LastIndex(signature, ")")
	if closeParen == -1 {
		return signatureStyle.Render(signature)
	}

	// If no parameters, just render the signature
	if len(params) == 0 {
		return signatureNameStyle.Render(funcName) +
			signatureStyle.Render("()")
	}

	// Build the signature with highlighted current parameter
	var b strings.Builder
	b.WriteString(signatureNameStyle.Render(funcName))
	b.WriteString(signatureStyle.Render("("))

	for i, param := range params {
		if i > 0 {
			b.WriteString(signatureSeparatorStyle.Render(", "))
		}

		// Check if this is a variadic parameter
		isVariadic := strings.HasPrefix(param, "...")

		// Highlight the current parameter
		// For variadic parameters, highlight if we're at or beyond that index
		if (isVariadic && currentArgIdx >= i) ||
			(!isVariadic && currentArgIdx == i) {
			b.WriteString(currentParamStyle.Render(param))
		} else {
			b.WriteString(signatureStyle.Render(param))
		}
	}

	b.WriteString(signatureStyle.Render(")"))

	return b.String()
}
