package lang

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

	"github.com/ardnew/aenv/log"
)

// runtimeEnvPool pools runtime environment maps to reduce allocations.
var runtimeEnvPool = sync.Pool{
	New: func() any {
		// Pre-allocate with reasonable capacity
		// Typical env has ~20 builtins + namespace count
		return make(map[string]any, 32)
	},
}

// programCache caches compiled expr programs to avoid recompilation.
// Key is the source string; value is the compiled program.
var (
	programCacheMu sync.RWMutex
	programCache   = make(map[string]*vm.Program)
)

// compileExpr compiles an expression, using cache when possible.
func compileExpr(
	source string,
	env map[string]any,
	patcher expr.Option,
) (*vm.Program, error) {
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

	// Double-check after acquiring write lock (another goroutine may have added
	// it)
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

	processEnv := buildProcessEnvMap(local.processEnv)

	// Create evaluation context early to stabilise the cached merged namespace
	// list. Using the same *Value pointers throughout ensures cycle detection
	// works correctly across the entire evaluation chain.
	ectx := &evalContext{
		get:        func() context.Context { return ctx },
		ast:        a,
		processEnv: processEnv,
		params:     make(map[string]any), // populated after validation below
		logger:     logger,
	}

	// Find the target namespace in the merged list and establish its lexical
	// scope. The namespace can see itself (visible is inclusive of position i),
	// which enables self-recursion for parameterized namespaces while
	// preventing forward references to later-defined namespaces.
	merged := ectx.mergedNamespaces()

	var (
		ns      *Namespace
		nsIndex int
	)

	for i, n := range merged {
		if n.Name == name {
			ns = n
			nsIndex = i

			break
		}
	}

	if ns == nil {
		return nil, ErrNotDefined.
			With(slog.String("name", name))
	}

	ectx.visible = merged[:nsIndex+1]

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

	if logger.Logger != nil && logger.Enabled(ctx, slog.Level(log.LevelTrace)) {
		logger.TraceContext(
			ctx,
			"param bindings",
			slog.Any("params", sortedKeys(params)),
		)
	}

	ectx.params = params

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

	if logger.Logger != nil && logger.Enabled(ctx, slog.Level(log.LevelTrace)) {
		logger.TraceContext(
			ctx,
			"runtime env keys",
			slog.Any("keys", sortedKeys(runtimeEnv)),
		)
	}

	// Set up patchers
	patcher := &hyphenPatcher{
		namespaces: ectx.mergedNamespaces(),
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

	// If result is a function, return a FuncRef describing it.
	if isFunction(result) {
		result = NewFuncRef(source, funcRefSignature(source, result, a))
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
	merged     []*Namespace        // cached merged top-level namespaces
	visible    []*Namespace        // top-level namespaces in scope (nil = all merged)
}

// mergedNamespaces returns the deduplicated/merged top-level namespaces,
// computing and caching the result on first call. Caching is essential:
// mergeEntries creates new *Value pointers for merged blocks, and reusing
// the same pointers lets the cycle detection in resolveForEnv work correctly.
func (ctx *evalContext) mergedNamespaces() []*Namespace {
	if ctx.merged == nil {
		ctx.merged = mergeEntries(ctx.ast.Namespaces)
	}

	return ctx.merged
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

	if ctx.logger.Logger != nil && ctx.logger.Enabled(ctx.get(), slog.Level(log.LevelTrace)) {
		ctx.logger.TraceContext(
			ctx.get(),
			"eval expr",
			slog.String("source", source),
			slog.Any("env_keys", sortedKeys(env)),
		)
	}

	// Set up patchers
	patcher := &hyphenPatcher{
		namespaces: ctx.mergedNamespaces(),
		env:        env,
		logger:     ctx.logger,
	}

	// When evaluating with a lexically restricted scope (visible != nil),
	// bypass the program cache: a cached program compiled with a broader env
	// could silently succeed even when a forward-reference variable is absent,
	// defeating the forward-reference error. Recompiling against the current
	// env ensures the type-checker catches any out-of-scope identifiers.
	var (
		program *vm.Program
		err     error
	)

	if ctx.visible != nil {
		program, err = expr.Compile(source, expr.Env(env), expr.Patch(patcher))
	} else {
		program, err = compileExpr(source, env, expr.Patch(patcher))
	}

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
// Entries are evaluated sequentially in definition order. Each entry can only
// reference siblings defined before it (lexical scope) — forward references
// within the block produce a compilation error.
func (ctx *evalContext) evaluateBlock(v *Value) (map[string]any, error) {
	if v.Entries == nil {
		return make(map[string]any), nil
	}

	ctx.logger.TraceContext(
		ctx.get(),
		"eval block",
		slog.Int("entry_count", len(v.Entries)),
	)

	// Merge duplicates within the block (block+block → recursive merge,
	// others → last definition wins), preserving first-occurrence order.
	merged := mergeEntries(v.Entries)

	result := make(map[string]any, len(merged))

	for _, ns := range merged {
		// Build an entry context that inherits the outer scope and all sibling
		// results evaluated so far. Later siblings are intentionally absent,
		// enforcing lexical (sequential) scope within the block.
		entryParams := make(map[string]any, len(ctx.params)+len(result))
		maps.Copy(
			entryParams,
			ctx.params,
		) // outer params (e.g., from EvaluateNamespace)
		maps.Copy(entryParams, result) // siblings evaluated so far

		entryCtx := &evalContext{
			get:        ctx.get,
			ast:        ctx.ast,
			processEnv: ctx.processEnv,
			params:     entryParams,
			logger:     ctx.logger,
			resolving:  ctx.resolving,
			merged:     ctx.merged,
			visible:    ctx.visible, // inherit the outer top-level scope
		}

		if len(ns.Params) > 0 {
			// Parameterized block entry: expose as a callable in the result.
			// The function body evaluates with the outer top-level scope.
			result[ns.Name] = entryCtx.makeNamespaceFunc(ns, ctx.visible)

			continue
		}

		evaluated, err := entryCtx.evaluateValue(ns.Value)
		if err != nil {
			return nil, err
		}

		result[ns.Name] = evaluated
	}

	if ctx.logger.Logger != nil && ctx.logger.Enabled(ctx.get(), slog.Level(log.LevelTrace)) {
		ctx.logger.TraceContext(
			ctx.get(),
			"block result",
			slog.Any("scope_keys", sortedKeys(result)),
		)
	}

	return result, nil
}

// buildRuntimeEnv constructs the environment for expr execution.
// The returned map is from a pool and should be returned via
// returnRuntimeEnv().
func (ctx *evalContext) buildRuntimeEnv() map[string]any {
	// Get pooled map
	env, ok := runtimeEnvPool.Get().(map[string]any)
	if !ok {
		env = make(map[string]any, 32)
	}

	// Clear any leftover entries (should be empty, but ensure it)
	for k := range env {
		delete(env, k)
	}

	// Start with built-in environment (copy directly from singleton)
	maps.Copy(env, builtinEnv())

	// Add parameters first (they take precedence and shadow builtins)
	maps.Copy(env, ctx.params)

	// Determine which top-level namespaces are in scope.
	// When visible is nil (e.g., EvaluateExpr / REPL), all merged namespaces
	// are visible. Otherwise only the prefix slice up to and including the
	// target namespace is considered (lexical scope: each namespace sees only
	// those defined before it, plus itself for self-recursion).
	merged := ctx.mergedNamespaces()

	effective := ctx.visible
	if effective == nil {
		effective = merged
	}

	// Add in-scope top-level namespaces from the AST.
	// Merge duplicate definitions: block+block namespaces have their entries
	// merged recursively (later entries shadow earlier ones by name);
	// non-block duplicates use last-definition-wins.
	for j, ns := range effective {
		// Skip if already in env (shadowed by params)
		if _, exists := env[ns.Name]; exists {
			continue
		}

		// When a lexical scope is active (visible != nil), each namespace's
		// definition-point scope is the prefix of the full merged list up to
		// and including its own position. This enables self-recursion while
		// preventing forward references.
		//
		// When visible is nil (EvaluateExpr / REPL mode) no scoping is
		// applied: defScope stays nil, which resolveForEnv / makeNamespaceFunc
		// interpret as "full scope" — all namespaces visible.
		var defScope []*Namespace
		if ctx.visible != nil {
			defScope = merged[:j+1]
		}

		if len(ns.Params) > 0 {
			// Parameterized namespace: add as callable function.
			env[ns.Name] = ctx.makeNamespaceFunc(ns, defScope)
		} else {
			// Non-parameterized: resolve using its definition-point scope so
			// that forward references within the body produce an error (when
			// lexical scope is active) or succeed (REPL mode).
			resolveCtx := &evalContext{
				get:        ctx.get,
				ast:        ctx.ast,
				processEnv: ctx.processEnv,
				params:     ctx.params,
				logger:     ctx.logger,
				resolving:  ctx.resolving,
				merged:     ctx.merged,
				visible:    defScope,
			}
			env[ns.Name] = resolveCtx.resolveForEnv(ns.Value)
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
// defScope is the set of top-level namespaces visible at the namespace's
// definition point; the function body evaluates with that scope.
func (ctx *evalContext) makeNamespaceFunc(
	ns *Namespace,
	defScope []*Namespace,
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

		// Create a child evalContext with parameter bindings.
		// visible is set to defScope (the namespace's definition-point scope).
		// When defScope is nil (EvaluateExpr / REPL mode), visible is also nil,
		// which means the function body evaluates with unrestricted scope.
		childCtx := &evalContext{
			get:        ctx.get,
			ast:        ctx.ast,
			processEnv: ctx.processEnv,
			params:     params,
			logger:     ctx.logger,
			resolving:  ctx.resolving,
			merged:     ctx.merged,
			visible:    defScope,
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

	case *FuncRef:
		if val.Signature != "" {
			return "<func: " + val.Signature + ">"
		}

		return "<func: " + val.Name + ">"

	case []any:
		return formatSlice(val)

	case map[string]any:
		return formatMap(val)

	default:
		if isFunction(v) {
			return formatFuncValue("", v)
		}

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
// Keys are sorted lexicographically for deterministic output.
func formatMap(m map[string]any) string {
	if len(m) == 0 {
		return "{}"
	}

	keys := sortedKeys(m)
	parts := make([]string, len(keys))

	for i, k := range keys {
		v := m[k]

		var vFormatted string
		if isFunction(v) {
			// Pass the map key so the signature is derived with a meaningful
			// name rather than an empty string.
			vFormatted = formatFuncValue(k, v)
		} else {
			vFormatted = formatResultValue(v)
		}

		parts[i] = strconv.Quote(k) + ": " + vFormatted
	}

	return "{" + strings.Join(parts, ", ") + "}"
}

// sortedKeys returns a lexicographically sorted list of keys from a map
// for deterministic logging and output.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	slices.Sort(keys)

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
func enhanceFunctionError(err error, _ string, ast *AST) *Error {
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

// isFunction reports whether v is any callable Go value.
// It uses reflection so that all function types — including builtin helpers
// with concrete signatures (e.g. func() string) — are detected correctly,
// not just the variadic func(...any)(any,error) used for user namespaces.
func isFunction(v any) bool {
	if v == nil {
		return false
	}

	return reflect.TypeOf(v).Kind() == reflect.Func
}

// funcRefSignature derives a human-readable call signature for a function
// value. For user-defined parameterized namespaces the signature is built
// from the AST parameter list (e.g. "add(x, y)"). For builtin functions it
// delegates to [funcSignatureFromReflect].
func funcRefSignature(name string, result any, a *AST) string {
	// Prefer exact parameter names for user-defined namespaces.
	if ns, ok := a.GetNamespace(name); ok && len(ns.Params) > 0 {
		return formatNamespaceSignature(ns)
	}

	return funcSignatureFromReflect(name, result)
}

// funcSignatureFromReflect derives a call signature from a Go function value
// using reflection alone (no AST required). Non-variadic parameters become
// "_" placeholders (e.g. "upper(_)"). Zero-parameter functions produce
// "name()". If fn is not a function, name is returned unchanged.
func funcSignatureFromReflect(name string, fn any) string {
	t := reflect.TypeOf(fn)
	if t == nil || t.Kind() != reflect.Func {
		return name
	}

	numIn := t.NumIn()
	if t.IsVariadic() {
		numIn-- // count only non-variadic leading params
	}

	if numIn <= 0 {
		return name + "()"
	}

	params := make([]string, numIn)
	for i := range params {
		params[i] = "_"
	}

	return name + "(" + strings.Join(params, ", ") + ")"
}

// formatFuncValue formats an anonymous Go function value for display.
// When name is non-empty it is used as the function identifier;
// otherwise the output is simply "<func>".
func formatFuncValue(name string, fn any) string {
	if name == "" {
		return "<func>"
	}

	return "<func: " + funcSignatureFromReflect(name, fn) + ">"
}
