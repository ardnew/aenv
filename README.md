# aenv

Static environment generator.

----

[![Go Reference](https://pkg.go.dev/badge/github.com/ardnew/aenv.svg)](https://pkg.go.dev/github.com/ardnew/aenv)
[![Go Report Card](https://goreportcard.com/badge/github.com/ardnew/aenv)](https://goreportcard.com/report/github.com/ardnew/aenv)
![Version](https://img.shields.io/github/v/tag/ardnew/aenv?label=version&sort=semver)

`aenv` is a composable, expression-driven environment generator. It evaluates a
lightweight DSL to produce environment variables from one or more configuration
sources -- useful anywhere static environment definitions are needed.

See [CONTRIBUTING.md](CONTRIBUTING.md) for build and test instructions.

## Usage

### Configuration

The DSL maps identifiers to expressions or nested blocks. Expressions are
evaluated by [expr-lang/expr](https://github.com/expr-lang/expr) and terminated
with a semicolon. Blocks group related namespaces.

```json
port : 8080;
host : "localhost";
url  : "http://" + host + ":" + string(port);

paths : {
    bin : path.cat(env.HOME, ".local", "bin");
    lib : path.cat(env.HOME, ".local", "lib");
}
```

Namespaces can accept parameters (including variadic) and can be composed of other user-defined namespaces, enabling functional-style composition of groups of environment variables:

```json
greet name     : "Hello, " + name;
sum   ...nums  : nums[0] + nums[1];
```

In addition to [everything provided by `expr-lang/expr`](https://expr-lang.org/docs/language-definition), many other builtin functions and namespaces are automatically exported for all expressions.

### Use Cases

  > [!NOTE]
  > Add examples

### Optimization

`aenv` is designed to be fast. It compiles the configuration into an efficient internal representation, and evaluates it in a single pass. The resulting environment is cached for subsequent runs, so the configuration is only parsed and evaluated once.

## Installation

### GitHub Releases

Download a prebuilt binary from the
[Releases](https://github.com/ardnew/aenv/releases) page.

### Go Install

```bash
go install github.com/ardnew/aenv@latest
```
