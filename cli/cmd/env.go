package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"strconv"
	"strings"

	"github.com/ardnew/aenv/lang"
	"github.com/ardnew/aenv/log"
)

// quote controls the value quoting style for env output.
type quote string

const (
	quoteNone   quote = "none"
	quoteSingle quote = "single"
	quoteDouble quote = "double"
	quotePosix  quote = "posix"
)

// wrap applies the selected quoting style to a value string.
func (q quote) wrap(val string) string {
	switch q {
	case quoteNone:
		return val
	case quoteSingle:
		return "'" + strings.ReplaceAll(val, "'", `'"'"'`) + "'"
	case quoteDouble:
		return strconv.Quote(val)
	case quotePosix:
		return "$'" + posixEscape(val) + "'"
	}

	return val
}

// Env evaluates a namespace and prints its members as sh-style environment
// variable definitions of the form KEY=VAL, one per line.
type Env struct {
	Name  string   `arg:"" help:"Namespace identifier to evaluate"`
	Args  []string `arg:"" help:"Parameters for parametric namespaces" optional:""`
	Quote quote    `       help:"Value quoting style"                              default:"none" enum:"none,single,double,posix"`
}

// Run executes the env command.
func (e *Env) Run(ctx context.Context) (err error) {
	ctx, cancel := context.WithCancelCause(ctx)

	defer func(err *error) { cancel(*err) }(&err)

	logger := log.With(slog.String("cmd", "env"))
	logger.TraceContext(ctx, "env start",
		slog.String("name", e.Name),
		slog.Int("arg_count", len(e.Args)),
	)

	var reader io.Reader

	if sf := sourceFilesFrom(ctx); sf != nil {
		reader = sf
	} else {
		reader = strings.NewReader("")
	}

	ast, err := lang.ParseReader(ctx, reader)
	if err != nil {
		return lang.WrapError(err).
			With(slog.String("command", "env"))
	}

	result, err := ast.EvaluateNamespace(ctx, e.Name, e.Args)
	if err != nil {
		return lang.WrapError(err).
			With(
				slog.String("command", "env"),
				slog.String("namespace", e.Name),
			)
	}

	// Get definition-order entries from the AST for ordered output
	var entries []*lang.Namespace
	if ns, ok := ast.GetNamespace(e.Name); ok && ns.Value != nil {
		entries = ns.Value.Entries
	}

	lines := collectEnv(e.Name, result, entries, e.Quote)
	for _, line := range lines {
		fmt.Println(line)
	}

	return nil
}

// collectEnv produces KEY=VAL lines from an evaluated namespace result.
// When the result is a map, entries are emitted in the definition order given
// by the namespace's block entries. Non-scalar values (maps, functions) are
// silently omitted.
func collectEnv(
	name string,
	result any,
	entries []*lang.Namespace,
	q quote,
) []string {
	m, ok := result.(map[string]any)
	if !ok {
		// Scalar namespace -- emit single line if it's a concrete value
		if isEnvScalar(result) {
			return []string{name + "=" + q.wrap(formatEnvValue(result))}
		}

		return nil
	}

	// Block namespace -- iterate entries in definition order, deduplicate
	var lines []string

	seen := make(map[string]struct{}, len(entries))
	for _, ns := range entries {
		if _, dup := seen[ns.Name]; dup {
			continue
		}

		seen[ns.Name] = struct{}{}

		val, exists := m[ns.Name]
		if !exists {
			continue
		}

		if !isEnvScalar(val) {
			continue // skip blocks, functions, FuncRefs
		}

		lines = append(lines, ns.Name+"="+q.wrap(formatEnvValue(val)))
	}

	return lines
}

// isEnvScalar checks whether a value is a concrete scalar suitable for env
// output (not a map, function, or [lang.FuncRef]).
func isEnvScalar(v any) bool {
	if v == nil {
		return true // nil produces empty value
	}

	switch v.(type) {
	case map[string]any:
		return false
	case *lang.FuncRef:
		return false
	default:
		if reflect.TypeOf(v).Kind() == reflect.Func {
			return false
		}

		return true
	}
}

// formatEnvValue formats a scalar value for env output. Strings are emitted
// raw (quoting is handled by [quote.wrap]). Arrays are joined with commas.
func formatEnvValue(val any) string {
	switch v := val.(type) {
	case nil:
		return ""
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		return v // raw string, no quoting
	case []any:
		parts := make([]string, len(v))
		for i, elem := range v {
			parts[i] = formatEnvValue(elem)
		}

		return strings.Join(parts, ",")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// posixEscape escapes a string for use inside POSIX $'...' quoting,
// replacing backslash, single quote, and C0 control characters with
// their backslash-escaped equivalents.
func posixEscape(s string) string {
	var b strings.Builder

	b.Grow(len(s))

	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}
