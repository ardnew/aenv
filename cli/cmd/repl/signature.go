package repl

import (
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"

	"github.com/ardnew/aenv/lang"
)

// exprLangBuiltins defines signatures for expr-lang's builtin functions.
// Source: https://expr-lang.org/docs/language-definition
var exprLangBuiltins = map[string]struct {
	signature string
	params    []string
}{
	"len":    {"len(v)", []string{"v"}},
	"all":    {"all(array, predicate)", []string{"array", "predicate"}},
	"any":    {"any(array, predicate)", []string{"array", "predicate"}},
	"one":    {"one(array, predicate)", []string{"array", "predicate"}},
	"none":   {"none(array, predicate)", []string{"array", "predicate"}},
	"map":    {"map(array, mapper)", []string{"array", "mapper"}},
	"filter": {"filter(array, predicate)", []string{"array", "predicate"}},
	"find":   {"find(array, predicate)", []string{"array", "predicate"}},
	"findIndex": {
		"findIndex(array, predicate)",
		[]string{"array", "predicate"},
	},
	"findLast": {
		"findLast(array, predicate)",
		[]string{"array", "predicate"},
	},
	"findLastIndex": {
		"findLastIndex(array, predicate)",
		[]string{"array", "predicate"},
	},
	"groupBy": {"groupBy(array, mapper)", []string{"array", "mapper"}},
	"sortBy":  {"sortBy(array, mapper)", []string{"array", "mapper"}},
	"count":   {"count(array, predicate)", []string{"array", "predicate"}},
	"sum":     {"sum(array)", []string{"array"}},
	"mean":    {"mean(array)", []string{"array"}},
	"median":  {"median(array)", []string{"array"}},
	"min":     {"min(array)", []string{"array"}},
	"max":     {"max(array)", []string{"array"}},
	"join":    {"join(array, separator)", []string{"array", "separator"}},
	"split": {
		"split(string, separator)",
		[]string{"string", "separator"},
	},
	"replace": {
		"replace(string, old, new)",
		[]string{"string", "old", "new"},
	},
	"trim":      {"trim(string)", []string{"string"}},
	"trimLeft":  {"trimLeft(string)", []string{"string"}},
	"trimRight": {"trimRight(string)", []string{"string"}},
	"upper":     {"upper(string)", []string{"string"}},
	"lower":     {"lower(string)", []string{"string"}},
	"title":     {"title(string)", []string{"string"}},
	"int":       {"int(v)", []string{"v"}},
	"float":     {"float(v)", []string{"v"}},
	"string":    {"string(v)", []string{"v"}},
	"type":      {"type(v)", []string{"v"}},
}

// ExprLangBuiltinNames returns the names of all expr-lang builtin functions.
func ExprLangBuiltinNames() []string {
	names := make([]string, 0, len(exprLangBuiltins))
	for name := range exprLangBuiltins {
		names = append(names, name)
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

	// Check expr-lang builtin functions
	if builtin, ok := exprLangBuiltins[funcName]; ok {
		return builtin.signature, builtin.params
	}

	// Check built-in functions using reflection
	if sig, params, ok := getBuiltinSignature(funcName); ok {
		return sig, params
	}

	return "", nil
}

// getBuiltinSignature uses reflection to extract the signature of a builtin
// function from the lang environment cache. Returns (signature, params, true)
// if found, ("", nil, false) otherwise.
func getBuiltinSignature(funcName string) (string, []string, bool) {
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
	var params []string

	numParams := t.NumIn()
	isVariadic := t.IsVariadic()

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
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "int"
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		return "uint"
	case reflect.Float32, reflect.Float64:
		return "float"
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

		return "arg"
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
