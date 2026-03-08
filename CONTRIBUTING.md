# Contributing

## Clone and Branch

```bash
git clone https://github.com/ardnew/aenv.git
cd aenv

# Feature branch
git checkout -b feature/my-feature

# Or use a worktree
git worktree add ../aenv-my-feature -b feature/my-feature
```

## Build

```bash
go build -v
```

## Build Tags

- `pprof` — Enable profiling support

> [!IMPORTANT]
> The `pprof` build tag may impact performance. Do *not* enable it when testing,
> benchmarking, or building for release. Use the Go toolchain's built-in profiling support for tests and benchmarks instead (see: [Profile](#profile) below).

## Test

```bash
go test -v -run=. ./...            # all
go test -v -run=. ./lang           # package
go test -v -run=TestName ./lang    # specific
```

## Benchmark

```bash
go test -v -bench=. -benchmem ./...                 # all
go test -v -bench=. -benchmem ./lang                # package
go test -v -bench=BenchmarkName -benchmem ./lang    # specific
```

## Profile

Normally, you can generate and analyze performance profiles using the tools built into the Go toolchain, but only for test cases and benchmarks:

```bash
go test -v -bench=. -benchmem -cpuprof=cpu.out -memprof=mem.out ./...
go tool pprof -http=:8080 ./aenv cpu.out
go tool pprof -http=:8080 ./aenv mem.out
```

This project adds profiling support to the main `aenv` application, not just
tests and benchmarks. It is useful for profiling real-world use, especially the
REPL, which is hard to analyze with standard tools. Because of the overhead
introduced, it is enabled only when built with the `pprof` build tag.

```bash
# Build with profiling support
go build -v -tags=pprof -o=./aenv

# Generate a profile
./aenv --pprof-mode=cpu       # CPU usage
./aenv --pprof-mode=allocs    # Memory allocations
./aenv --pprof-mode=trace     # Execution trace

# Launch the pprof HTTP server for live analysis
./aenv eval --pprof-mode=cpu --pprof-http=:6060

# Analyze interactively
go tool pprof -http=:6060 ./aenv "${XDG_CACHE_HOME}/aenv/pprof/<mode>.pprof"
```

## Coverage

```bash
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Lint

```bash
golangci-lint run --config=.golangci.yml --fix
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
git commit -m "feat: describe change"
git push origin feature/my-feature
```

Open a pull request against `main` on GitHub.
