# lang Package

Recursive descent parser for the aenv configuration language.

## Design

- Constructs an AST composed of **Namespaces**, read from one or more input sources (files, strings, etc.), parsed in natural order.
- A **Namespace** associates a **Value** with an identifier and optional **Parameters**.
- **Parameters** are simple identifiers, and the last **Parameter** can be variadic (e.g. `...identifier`).
- A **Value** can be one of two kinds:
  - **Block**: Zero or more **Namespaces** enclosed in curly braces `{…}`.
  - **Expression**: A string that will be evaluated by [expr-lang/expr](https://github.com/expr-lang/expr), terminated by semi-colon `;`.

### Architecture

```
lang/
├── doc.go        # Package documentation and grammar
├── ast.go        # Core AST types (Namespace, Value, Param, Position)
├── error.go      # Structured error handling
├── parse.go      # Hand-written recursive descent parser
├── env.go        # Built-in environment (target, platform, cwd, file.*, path.*, mung.*)
├── value.go      # Programmatic value constructors
├── patcher.go    # AST visitors for expr-lang integration
├── eval.go       # Evaluation engine with expr-lang
├── format.go     # Pretty-printing (native, JSON, YAML)
└── marshal.go    # JSON/YAML marshaling
```

## Grammar

```ebnf
Manifest   = Namespace (Sep Namespace)* Sep?
Namespace  = Identifier Params? ':' Value
Params     = Identifier | ('...' Identifier) | (Identifier Params)
Value      = Block | Expression
Block      = '{' (Namespace (Sep Namespace)* Sep?)? '}'
Expression = <any text except '{' at start>
Separator  = ';'
Identifier = [a-zA-Z_][a-zA-Z0-9_]* ('-' | '+' | '.' | '@' | '/' [a-zA-Z0-9_]+)*
Comment    = '//' <text until newline>
```

## Usage

### Basic Parsing

```go
package main

import (
    "context"
    "fmt"
    "github.com/ardnew/aenv/lang"
)

func main() {
    input := `
        port : 8080
        host : "localhost"
        url : "http://" + host + ":" + string(port)
    `

    ast, err := lang.ParseString(context.Background(), input)
    if err != nil {
        panic(err)
    }

    result, err := ast.EvaluateNamespace(context.Background(), "url", nil)
    if err != nil {
        panic(err)
    }

    fmt.Println(result) // Output: http://localhost:8080
}
```

### Parameterized Namespaces

```go
input := `greet name : "Hello, " + name`

ast, _ := lang.ParseString(context.Background(), input)
result, _ := ast.EvaluateNamespace(context.Background(), "greet", []string{"Alice"})

fmt.Println(result) // Output: Hello, Alice
```

### Variadic Parameters

```go
input := `sum ...nums : nums[0] + nums[1] + nums[2]`

ast, _ := lang.ParseString(context.Background(), input)
result, _ := ast.EvaluateNamespace(context.Background(), "sum", []string{"10", "20", "30"})

fmt.Println(result) // Output: 60
```

### Blocks with Scoping

```go
input := `
    config : {
        port : 8080,
        url : "http://localhost:" + string(port)
    }
`

ast, _ := lang.ParseString(context.Background(), input)
result, _ := ast.EvaluateNamespace(context.Background(), "config", nil)

m := result.(map[string]any)
fmt.Println(m["url"]) // Output: http://localhost:8080
```

### Built-in Functions

```go
// Environment variables
home : env("HOME")

// Platform information
os : platform.OS
arch : platform.Arch

// Current working directory
dir : cwd()

// File system checks
exists : file.exists("/path/to/file")
isdir : file.isDir("/path")

// Path manipulation
abs : path.abs("relative/path")
joined : path.cat("foo", "bar", "baz.txt")

// PATH-like manipulation via mung
newpath : mung.prefix(env("PATH"), "/usr/local/bin")
```

### Formatting

```go
// Native syntax
ast.Format(ctx, os.Stdout, 2) // indent=2

// JSON
ast.FormatJSON(ctx, os.Stdout, 2)

// YAML
ast.FormatYAML(ctx, os.Stdout, 2)

// Debug representation
ast.Print(ctx, os.Stdout)
```

## API Reference

### Core Types

```go
type AST struct {
    Namespaces []*Namespace
    // ...
}

type Namespace struct {
    Name   string
    Params []Param
    Value  *Value
    Pos    Position
}

type Value struct {
    Kind    ValueKind    // KindExpr or KindBlock
    Source  string       // Raw expression (KindExpr)
    Entries []*Namespace // Block entries (KindBlock)
    Pos     Position
}

type Param struct {
    Name     string
    Variadic bool // true for ...name
}
```

### Parsing

```go
// Parse from string
func ParseString(ctx context.Context, s string, opts ...Option) (*AST, error)

// Parse from reader
func ParseReader(ctx context.Context, r io.Reader, opts ...Option) (*AST, error)
```

### Options

```go
// Set logger for trace debugging
WithLogger(logger log.Logger) Option

// Override process environment for env() function
WithProcessEnv(env []string) Option
```

### Evaluation

```go
// Evaluate a namespace with arguments
func (a *AST) EvaluateNamespace(
    ctx context.Context,
    name string,
    args []string,
    opts ...Option,
) (any, error)

// Evaluate an expression directly
func (a *AST) EvaluateExpr(
    ctx context.Context,
    source string,
    opts ...Option,
) (any, error)

// Format evaluation result
func FormatResult(result any) string
```

### Formatting

```go
// Format as native syntax
func (a *AST) Format(ctx context.Context, w io.Writer, indent int) error

// Format as JSON
func (a *AST) FormatJSON(ctx context.Context, w io.Writer, indent int) error

// Format as YAML
func (a *AST) FormatYAML(ctx context.Context, w io.Writer, indent int) error

// Debug print AST structure
func (a *AST) Print(ctx context.Context, w io.Writer) error
```

### AST Manipulation

```go
// Get namespace by name
func (a *AST) GetNamespace(name string) (*Namespace, bool)

// Iterate all namespaces
func (a *AST) All() iter.Seq[*Namespace]

// Define or replace namespace
func (a *AST) DefineNamespace(name string, params []Param, value *Value)

// Convert to Go map
func (a *AST) ToMap() map[string]any
```

### Value Constructors

```go
// Create expression value
func NewExpr(source string) *Value

// Create block value
func NewBlock(entries ...*Namespace) *Value

// Create namespace
func NewNamespace(name string, params []Param, value *Value) *Namespace
```

## Migration from lang to lang2

### Syntax Changes

1. **Remove double braces from expressions:**

   ```diff
   - port : {{ 8080 }}
   + port : 8080

   - url : {{ "http://" + host }}
   + url : "http://" + host
   ```

2. **Use expr-lang syntax for arrays:**

   ```diff
   - ports : { 80, 443, 8080 }
   + ports : [80, 443, 8080]
   ```

3. **Blocks only contain namespaces (maps):**

   ```diff
   - config : { 80, "localhost" }  # Not allowed
   + config : { port : 80; host : "localhost" }
   ```

### API Changes

1. **DefineNamespace signature:**

   ```diff
   - ast.DefineNamespace(name, params, variadic, value)
   + ast.DefineNamespace(name, params, value)
   ```

   The `variadic` flag is now part of the last Param.

2. **Value constructors:**

   ```diff
   - lang.NewString("hello")
   - lang.NewNumber("42")
   - lang.NewBool(true)
   + lang.NewExpr(`"hello"`)
   + lang.NewExpr("42")
   + lang.NewExpr("true")
   ```

3. **Tuple → Block:**

   ```diff
   - lang.NewTuple(entries...)
   + lang.NewBlock(entries...)
   ```

## Testing

Run the comprehensive test suite:

```bash
go test -v ./lang2/...
```

All tests include:

- Parse tests (simple, parameters, blocks, errors)
- Evaluation tests (literals, arithmetic, blocks, parameters, builtins)
- Format tests (native, JSON, YAML, round-trips)

## Performance

The hand-written parser is faster than the generated GLL parser and uses less memory:

- No parse forest construction
- Direct AST building
- Single-pass parsing
- Minimal allocations

## Future Work

- [ ] CLI integration (update import paths)
- [ ] Migration script for existing .aenv files
- [ ] Benchmarks comparing to lang package
- [ ] Additional built-in functions
- [ ] Watch mode for file changes

## License

Same as the parent aenv project.
