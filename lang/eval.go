package lang

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"strconv"
	"strings"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

	"github.com/ardnew/aenv/log"
)

// runtimeEnvPool pools runtime environment maps to reduce allocations.
//
//nolint:gochecknoglobals
var runtimeEnvPool = sync.Pool{
	New: func() any {
		// Pre-allocate with reasonable capacity
		// Typical env has ~20 builtins + namespace count
		return make(map[string]any, 32)
	},
}

// programCache caches compiled expr programs to avoid recompilation.
// Key is the source string; value is the compiled program.
//
//nolint:gochecknoglobals
var (
	programCacheMu sync.RWMutex
	programCache   = make(map[string]*vm.Program)
)

// compileExpr compiles an expression, using cache when possible.
func compileExpr(source string, env map[string]any, patcher expr.Option) (*vm.Program, error) {
	// Check cache first (read lock)
	programCacheMu.RLock()
	if prog, ok := programCache[source]; ok {
		programCacheMu.RUnlock()
		return prog, nil
	}
	programCacheMu.RUnlock()

	// Not in cache, compile it (write lock)
	programCacheMu.Lock()
	defer programCacheMu.Unlock()

	// Double-check after acquiring write lock (another goroutine may have added it)
	if prog, ok := programCache[source]; ok {
		return prog, nil
	}

	// Compile the expression
	program, err := expr.Compile(
		source,
		expr.Env(env),
		patcher,
	)
	if err != nil {
		return nil, err
	}

	// Add to cache (simple unbounded cache for now)
	programCache[source] = program

	return program, nil
}

// ClearProgramCache clears the compiled program cache.
// This should be called when AST is modified (e.g., after `:edit` in REPL).
func ClearProgramCache() {
	programCacheMu.Lock()
	defer programCacheMu.Unlock()
	programCache = make(map[string]*vm.Program)
}

// EvaluateNamespace evaluates a namespace with the given parameter bindings.
// The args slice provides values for each parameter in order.
// Returns the fully evaluated result.
//
// Options are applied as overrides to ast.opts for this evaluation only,
// enabling thread-safe concurrent evaluations with different processEnv.
func (a *AST) EvaluateNamespace(
	ctx context.Context,
	name string,
	args []string,
	opts ...Option,
) (any, error) {
	// Copy ast.opts locally for thread-safe concurrent evaluation
	local := a.opts
	logger := a.logger

	for _, opt := range opts {
		// Apply option to a temporary AST to extract the setting
		var temp AST

		temp.opts = local
		temp.logger = logger
		opt(&temp)
		local = temp.opts
		logger = temp.logger
	}

	logger.TraceContext(
		ctx,
		"eval namespace",
		slog.String("name", name),
		slog.Int("arg_count", len(args)),
	)

	ns, found := a.GetNamespace(name)
	if !found {
		return nil, ErrNotDefined.
			With(slog.String("name", name))
	}

	// Check if namespace is parameterized
	if len(ns.Params) == 0 && len(args) > 0 {
		return nil, ErrParameterCount.
			With(
				slog.String("name", name),
				slog.Int("expected", 0),
				slog.Int("got", len(args)),
			)
	}

	// Validate argument count
	var variadic bool
	if len(ns.Params) > 0 {
		variadic = ns.Params[len(ns.Params)-1].Variadic
	}

	if variadic {
		required := len(ns.Params) - 1
		if len(args) < required {
			return nil, ErrParameterCount.
				With(
					slog.String("name", name),
					slog.Int("min_expected", required),
					slog.Int("got", len(args)),
				)
		}
	} else if len(args) != len(ns.Params) {
		return nil, ErrParameterCount.
			With(
				slog.String("name", name),
				slog.Int("expected", len(ns.Params)),
				slog.Int("got", len(args)),
			)
	}

	processEnv := buildProcessEnvMap(local.processEnv)

	// Build parameter bindings map by parsing each argument as an expression.
	// For variadic namespaces, the last parameter collects all remaining
	// arguments into a slice.
	params := make(map[string]any, len(ns.Params))

	last := len(ns.Params) - 1
	for i, param := range ns.Params {
		if variadic && i == last {
			// Collect remaining args into a slice
			rest := make([]any, 0, len(args)-i)
			for _, arg := range args[i:] {
				rest = append(rest, parseArgToAny(arg))
			}

			params[param.Name] = rest
		} else {
			params[param.Name] = parseArgToAny(args[i])
		}
	}

	logger.TraceContext(
		ctx,
		"param bindings",
		slog.Any("params", sortedKeys(params)),
	)

	// Build evaluation context
	ectx := &evalContext{
		get:        func() context.Context { return ctx },
		ast:        a,
		processEnv: processEnv,
		params:     params,
		logger:     logger,
	}

	return ectx.evaluateValue(ns.Value)
}

// EvaluateExpr compiles and executes an expr-lang expression using the AST's
// namespaces as the environment. Simple namespace names resolve as identifiers,
// parameterized namespaces are callable functions, and the env() builtin is
// available.
//
// Options are applied as overrides to ast.opts for this evaluation only.
func (a *AST) EvaluateExpr(
	ctx context.Context,
	source string,
	opts ...Option,
) (any, error) {
	// Copy ast.opts locally for thread-safe concurrent evaluation
	local := a.opts
	logger := a.logger

	for _, opt := range opts {
		var temp AST

		temp.opts = local
		temp.logger = logger
		opt(&temp)
		local = temp.opts
		logger = temp.logger
	}

	processEnv := buildProcessEnvMap(local.processEnv)

	logger.TraceContext(
		ctx,
		"eval expr",
		slog.String("source", source),
	)

	// Build evaluation context
	ectx := &evalContext{
		get:        func() context.Context { return ctx },
		ast:        a,
		processEnv: processEnv,
		params:     make(map[string]any),
		logger:     logger,
	}

	// Build runtime environment with actual resolved values
	runtimeEnv := ectx.buildRuntimeEnv()
	defer returnRuntimeEnv(runtimeEnv)

	logger.TraceContext(
		ctx,
		"runtime env keys",
		slog.Any("keys", sortedKeys(runtimeEnv)),
	)

	// Set up patchers
	patcher := &hyphenPatcher{
		namespaces: a.Namespaces,
		env:        runtimeEnv,
		logger:     logger,
	}

	program, err := compileExpr(source, runtimeEnv, expr.Patch(patcher))
	if err != nil {
		return nil, ErrExprCompile.Wrap(err).
			With(slog.String("source", source))
	}

	result, err := vm.Run(program, runtimeEnv)
	if err != nil {
		return nil, enhanceFunctionError(err, source, a).
			With(slog.String("source", source))
	}

	// If result is a function, return nil (like expr-lang builtins)
	if isFunction(result) {
		result = nil
	}

	logger.TraceContext(
		ctx,
		"expr result",
		slog.String("result_type", resultTypeName(result)),
	)

	return result, nil
}

// evalContext holds the state for recursive evaluation.
type evalContext struct {
	get        func() context.Context // allows for cancellation and timeouts
	ast        *AST
	processEnv map[string]string
	params     map[string]any      // parameter bindings
	logger     log.Logger          // structured logger
	resolving  map[*Value]struct{} // cycle detection for resolveForEnv
}

// evaluateValue recursively evaluates a Value to its Go representation.
func (ctx *evalContext) evaluateValue(v *Value) (any, error) {
	if v == nil {
		return nil, nil
	}

	ctx.logger.TraceContext(
		ctx.get(),
		"eval dispatch",
		slog.String("value_kind", v.Kind.String()),
	)

	switch v.Kind {
	case KindExpr:
		return ctx.evaluateExpr(v)

	case KindBlock:
		return ctx.evaluateBlock(v)

	default:
		return nil, ErrInvalidValueType.
			With(slog.String("kind", v.Kind.String()))
	}
}

// evaluateExpr evaluates an expression value by compiling and running it
// with expr-lang.
func (ctx *evalContext) evaluateExpr(v *Value) (any, error) {
	source := v.Source

	// Skip empty expressions
	if source == "" {
		return "", nil
	}

	// Build runtime environment with resolved parameter values
	env := ctx.buildRuntimeEnv()
	defer returnRuntimeEnv(env)

	ctx.logger.TraceContext(
		ctx.get(),
		"eval expr",
		slog.String("source", source),
		slog.Any("env_keys", sortedKeys(env)),
	)

	// Set up patchers
	patcher := &hyphenPatcher{
		namespaces: ctx.ast.Namespaces,
		env:        env,
		logger:     ctx.logger,
	}

	program, err := compileExpr(source, env, expr.Patch(patcher))
	if err != nil {
		return nil, ErrExprCompile.Wrap(err).
			With(slog.String("source", source))
	}

	result, err := vm.Run(program, env)
	if err != nil {
		return nil, enhanceFunctionError(err, source, ctx.ast).
			With(slog.String("source", source))
	}

	// If result is a function, return nil (like expr-lang builtins)
	if isFunction(result) {
		result = nil
	}

	ctx.logger.TraceContext(
		ctx.get(),
		"expr vm result",
		slog.String("result_type", resultTypeName(result)),
	)

	return result, nil
}

// evaluateBlock evaluates a block value to a map[string]any.
// The block's namespace entries can reference each other (sibling scope).
func (ctx *evalContext) evaluateBlock(v *Value) (map[string]any, error) {
	if v.Entries == nil {
		return make(map[string]any), nil
	}

	ctx.logger.TraceContext(
		ctx.get(),
		"eval block",
		slog.Int("entry_count", len(v.Entries)),
	)

	// Create a child context with the block's namespace scope.
	// This allows expressions within the block to reference sibling namespaces.
	childCtx := &evalContext{
		get:        ctx.get,
		ast:        ctx.ast,
		processEnv: ctx.processEnv,
		params:     make(map[string]any),
		logger:     ctx.logger,
		resolving:  ctx.resolving,
	}

	// Copy parent params
	maps.Copy(childCtx.params, ctx.params)

	// Add block-scoped namespaces to child context as parameters.
	// This enables sibling references like: { x : 1; y : x + 1 }
	// Only add non-parameterized namespaces to avoid circular evaluation.
	for _, ns := range v.Entries {
		if len(ns.Params) == 0 {
			// Add as a resolvable parameter
			childCtx.params[ns.Name] = childCtx.resolveForEnv(ns.Value)
		}
	}

	ctx.logger.TraceContext(
		ctx.get(),
		"block scope",
		slog.Any("scope_keys", sortedKeys(childCtx.params)),
	)

	// Evaluate all namespaces with the enriched context
	result := make(map[string]any, len(v.Entries))

	for _, ns := range v.Entries {
		evaluated, err := childCtx.evaluateValue(ns.Value)
		if err != nil {
			return nil, err
		}

		result[ns.Name] = evaluated
	}

	return result, nil
}

// buildRuntimeEnv constructs the environment for expr execution.
// The returned map is from a pool and should be returned via returnRuntimeEnv().
func (ctx *evalContext) buildRuntimeEnv() map[string]any {
	// Get pooled map
	env := runtimeEnvPool.Get().(map[string]any)

	// Clear any leftover entries (should be empty, but ensure it)
	for k := range env {
		delete(env, k)
	}

	// Start with built-in environment (copy into pooled map)
	builtins := makeEnvCache()
	maps.Copy(env, builtins)

	// Add parameters first (they take precedence and shadow builtins)
	maps.Copy(env, ctx.params)

	// Add top-level namespaces from the AST.
	// For non-parameterized namespaces, we try to resolve their values.
	// For parameterized namespaces, we add them as callable functions.
	// Namespace patching will optimize constant-valued namespaces.
	resolved := make(map[string]any)

	for _, ns := range ctx.ast.Namespaces {
		// Skip if already in env (shadowed by params)
		if _, exists := env[ns.Name]; exists {
			continue
		}

		if len(ns.Params) > 0 {
			// Parameterized namespace: add as callable function
			env[ns.Name] = ctx.makeNamespaceFunc(ns)
		} else {
			// Non-parameterized: resolve value for patching
			val := ctx.resolveForEnv(ns.Value)
			env[ns.Name] = val
			resolved[ns.Name] = val
		}
	}

	// Add env map for access to process environment variables
	env["env"] = ctx.processEnv

	ctx.logger.TraceContext(
		ctx.get(),
		"runtime env",
		slog.Int("param_count", len(ctx.params)),
		slog.Int("namespace_count", len(ctx.ast.Namespaces)),
		slog.Int("total_keys", len(env)),
	)

	return env
}

// returnRuntimeEnv returns a runtime environment map to the pool.
func returnRuntimeEnv(env map[string]any) {
	// Clear the map before returning to pool
	for k := range env {
		delete(env, k)
	}
	runtimeEnvPool.Put(env)
}

// makeNamespaceFunc creates a callable function for a parameterized namespace.
// This allows parameterized namespaces to be called from within expressions.
func (ctx *evalContext) makeNamespaceFunc(
	ns *Namespace,
) func(...any) (any, error) {
	return func(args ...any) (any, error) {
		ctx.logger.TraceContext(
			ctx.get(),
			"call namespace func",
			slog.String("name", ns.Name),
			slog.Int("arg_count", len(args)),
		)

		// Check if namespace is variadic
		var variadic bool
		if len(ns.Params) > 0 {
			variadic = ns.Params[len(ns.Params)-1].Variadic
		}

		// Validate argument count
		if variadic {
			required := len(ns.Params) - 1
			if len(args) < required {
				return nil, ErrArgumentCount.
					With(
						slog.String("name", ns.Name),
						slog.String("signature", formatNamespaceSignature(ns)),
						slog.Int("min_expected", required),
						slog.Int("got", len(args)),
					)
			}
		} else if len(args) != len(ns.Params) {
			return nil, ErrArgumentCount.
				With(
					slog.String("name", ns.Name),
					slog.String("signature", formatNamespaceSignature(ns)),
					slog.Int("expected", len(ns.Params)),
					slog.Int("got", len(args)),
				)
		}

		// Build parameter bindings
		params := make(map[string]any, len(ns.Params))

		last := len(ns.Params) - 1
		for i, param := range ns.Params {
			if variadic && i == last {
				// Collect remaining args into a slice
				params[param.Name] = args[i:]
			} else {
				params[param.Name] = args[i]
			}
		}

		// Create a child evalContext with parameter bindings
		childCtx := &evalContext{
			get:        ctx.get,
			ast:        ctx.ast,
			processEnv: ctx.processEnv,
			params:     params,
			logger:     ctx.logger,
			resolving:  ctx.resolving,
		}

		return childCtx.evaluateValue(ns.Value)
	}
}

// resolveForEnv converts a Value to a Go value suitable for the expr
// environment. This avoids infinite recursion by tracking values being
// resolved and returning nil for cycles.
func (ctx *evalContext) resolveForEnv(v *Value) any {
	if v == nil {
		return nil
	}

	// Cycle detection
	if ctx.resolving == nil {
		ctx.resolving = make(map[*Value]struct{})
	}

	if _, cycling := ctx.resolving[v]; cycling {
		return nil
	}

	ctx.resolving[v] = struct{}{}
	defer delete(ctx.resolving, v)

	switch v.Kind {
	case KindExpr:
		// Try to evaluate the expression
		result, err := ctx.evaluateValue(v)
		if err != nil {
			ctx.logger.TraceContext(
				ctx.get(),
				"resolve expression for environment",
				slog.String("source", v.Source),
				slog.String("error", err.Error()),
			)

			return nil
		}

		return result

	case KindBlock:
		// Resolve block to map
		result, err := ctx.evaluateBlock(v)
		if err != nil {
			ctx.logger.TraceContext(
				ctx.get(),
				"resolve block for environment",
				slog.String("error", err.Error()),
			)

			return nil
		}

		return result

	default:
		return nil
	}
}

// parseArgToAny converts a command-line argument string to a Go value.
// It uses smart inference to avoid requiring quoted strings:
//   - Booleans (true/false) → bool
//   - Numbers (integers, floats) → int64/float64
//   - Otherwise → string (treated as unquoted string)
func parseArgToAny(arg string) any {
	arg = strings.TrimSpace(arg)

	// Empty string
	if arg == "" {
		return ""
	}

	// Check for boolean literals
	if arg == "true" {
		return true
	}

	if arg == "false" {
		return false
	}

	// Check for number (try parsing as int or float)
	if i, err := strconv.ParseInt(arg, 0, 64); err == nil {
		return i
	}

	if f, err := strconv.ParseFloat(arg, 64); err == nil {
		return f
	}

	// Check for quoted string
	if (strings.HasPrefix(arg, `"`) && strings.HasSuffix(arg, `"`)) ||
		(strings.HasPrefix(arg, "'") && strings.HasSuffix(arg, "'")) ||
		(strings.HasPrefix(arg, "`") && strings.HasSuffix(arg, "`")) {
		if unquoted, err := strconv.Unquote(arg); err == nil {
			return unquoted
		}
	}

	// Default: treat as a plain string
	return arg
}

// FormatResult formats an evaluation result for output.
// This produces an expr-lang compatible syntax representation.
func FormatResult(result any) string {
	return formatResultValue(result)
}

// formatResultValue recursively formats a Go value as expr-lang syntax.
func formatResultValue(v any) string {
	switch val := v.(type) {
	case nil:
		return "nil"

	case bool:
		return strconv.FormatBool(val)

	case int:
		return strconv.Itoa(val)

	case int64:
		return strconv.FormatInt(val, 10)

	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)

	case string:
		// Always quote strings for expr-lang compatibility
		return strconv.Quote(val)

	case []any:
		return formatSlice(val)

	case map[string]any:
		return formatMap(val)

	default:
		return fmt.Sprintf("%v", val)
	}
}

// formatSlice formats a slice as expr-lang array syntax.
func formatSlice(vals []any) string {
	if len(vals) == 0 {
		return "[]"
	}

	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = formatResultValue(v)
	}

	return "[" + strings.Join(parts, ", ") + "]"
}

// formatMap formats a map as expr-lang map syntax.
func formatMap(m map[string]any) string {
	if len(m) == 0 {
		return "{}"
	}

	parts := make([]string, 0, len(m))

	for k, v := range m {
		parts = append(parts, strconv.Quote(k)+": "+formatResultValue(v))
	}

	return "{" + strings.Join(parts, ", ") + "}"
}

// sortedKeys returns a sorted list of keys from a map for deterministic
// logging.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	return keys
}

// formatNamespaceSignature formats a namespace signature with parameter names.
// Examples: "foo()", "bar(x, y)", "baz(a, ...rest)".
func formatNamespaceSignature(ns *Namespace) string {
	if len(ns.Params) == 0 {
		return ns.Name + "()"
	}

	params := make([]string, len(ns.Params))
	for i, p := range ns.Params {
		if p.Variadic {
			params[i] = "..." + p.Name
		} else {
			params[i] = p.Name
		}
	}

	return ns.Name + "(" + strings.Join(params, ", ") + ")"
}

// enhanceFunctionError checks if an error is related to a function call and
// adds signature information if available.
func enhanceFunctionError(err error, source string, ast *AST) *Error {
	// First check if the error chain contains ErrArgumentCount, which already
	// has signature information attached. If so, wrap it with ErrExprEvaluate
	// while preserving the attributes.
	var argCountErr *Error
	if errors.As(err, &argCountErr) && argCountErr != nil {
		// Check if this is specifically an ErrArgumentCount by comparing messages
		if argCountErr.msg == ErrArgumentCount.msg {
			// Preserve all attributes from the argument count error
			return ErrExprEvaluate.Wrap(err).
				With(argCountErr.attrs...)
		}
	}

	errMsg := err.Error()

	// Try to extract function name from common error patterns
	var funcName string

	// Pattern: "cannot call ... (type ...)"
	if idx := strings.Index(errMsg, "cannot call"); idx >= 0 {
		// Extract identifier after "cannot call"
		after := errMsg[idx+len("cannot call"):]
		if fields := strings.Fields(after); len(fields) > 0 {
			funcName = strings.Trim(fields[0], "'\"()")
		}
	}

	// Pattern: "unknown name ... ()"
	if funcName == "" {
		if idx := strings.Index(errMsg, "unknown name"); idx >= 0 {
			after := errMsg[idx+len("unknown name"):]
			if fields := strings.Fields(after); len(fields) > 0 {
				funcName = strings.Trim(fields[0], "'\"()")
			}
		}
	}

	// Pattern: "invalid number of arguments ... (expected ...)"
	// or "not enough arguments ... (expected ...)"
	if funcName == "" {
		for _, pattern := range []string{"arguments to", "arguments for"} {
			if idx := strings.Index(errMsg, pattern); idx >= 0 {
				after := errMsg[idx+len(pattern):]
				if fields := strings.Fields(after); len(fields) > 0 {
					funcName = strings.Trim(fields[0], "'\"()")
				}

				break
			}
		}
	}

	// If we found a function name, try to look it up
	if funcName != "" {
		if ns, ok := ast.GetNamespace(funcName); ok && len(ns.Params) > 0 {
			sig := formatNamespaceSignature(ns)

			return ErrExprEvaluate.Wrap(err).
				With(
					slog.String("function", funcName),
					slog.String("signature", sig),
				)
		}
	}

	// No function found, return basic error
	return ErrExprEvaluate.Wrap(err)
}

// isFunction checks if a value is a function type.
func isFunction(v any) bool {
	if v == nil {
		return false
	}

	// Check for function types that would be returned by expr-lang
	switch v.(type) {
	case func(...any) (any, error):
		return true
	default:
		return false
	}
}
