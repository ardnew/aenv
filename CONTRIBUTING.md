# Contributing

## Clone and Branch

```bash
git clone https://github.com/ardnew/aenv.git
cd aenv

# Use a worktree
git worktree add ../work/<context>/<change> -b <context>/<change>

# Or a traditional feature branch
git checkout -b <context>/<change>
```

## Build

```bash
go build -v
```

## Test

```bash
go test -v -run=. ./...            # all
go test -v -run=. ./<package>      # package
go test -v -run=<name> ./<package> # specific
```

## Benchmark

```bash
go test -v -bench=. -benchmem ./...            # all
go test -v -bench=. -benchmem ./<package>      # package
go test -v -bench=<name> -benchmem ./<package> # specific
```

## Profile

## Coverage

```bash
go test -v -coverprofile=<output> ./...
go tool cover -html=<output>
```

## Lint

```bash
golangci-lint run --config=.golangci.yaml --fix
```

### Requirements

- `golangci-lint`
  - Must use the newer v2 command:
    - `github.com/golangci/golangci-lint/v2/cmd/golangci-lint`
    - Some distros package this as `golangci-lint-v2`
  - See [installation instructions](https://golangci-lint.run/docs/welcome/install/local/)

## Conventions

- Follow [Effective Go](https://go.dev/doc/effective_go) and
  [Code Review Comments](https://go.dev/wiki/CodeReviewComments).
- Place package documentation in `doc.go` only, starting with `Package <name>`.
- Doc comments on exported symbols start with the symbol name.
- Test names follow `Test<Type>_<Method>_<Behavior>`.
- Use table-driven tests with `t.Run`.
- Linter suppressions go in `.golangci.yml`, never inline `//nolint`.
- No emoji in code, comments, or documentation.

## Submit

```bash
git add -A
git commit -m "<scope>: <brief>"
git push origin <context>/<change>
```

Open a pull request against `main` on GitHub.
