module github.com/ardnew/aenv

go 1.25.3

tool (
	github.com/goccmack/gogll/v3
	golang.org/x/tools/cmd/stringer
	golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment
)

require (
	github.com/alecthomas/kong v1.13.0
	github.com/goccmack/goutil v1.2.3
	github.com/goccy/go-yaml v1.19.1
	github.com/klauspost/readahead v1.4.0
	github.com/pkg/profile v1.7.0
	github.com/zeebo/xxh3 v1.1.0
)

require (
	github.com/felixge/fgprof v0.9.5 // indirect
	github.com/goccmack/gogll/v3 v3.4.1 // indirect
	github.com/google/pprof v0.0.0-20251114195745-4902fdda35c8 // indirect
	github.com/iancoleman/strcase v0.1.3 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	golang.org/x/mod v0.30.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/tools v0.39.0 // indirect
)

replace github.com/goccmack/gogll/v3 => github.com/ardnew/gogll/v3 v3.4.2-0.20260119000734-6ce9e2ac32fd
