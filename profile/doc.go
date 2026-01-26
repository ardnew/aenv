// Package profile provides optional runtime profiling for the aenv
// application.
//
// # Overview
//
// This package integrates [github.com/pkg/profile] to provide runtime profiling
// capabilities with conditional compilation support. Profiling is optional and
// must be enabled at build time using the "pprof" build tag.
//
// When built with profiling disabled (default), all operations are no-ops with
// zero runtime overhead.
//
// # Available Profiling Modes
//
// The following profiling modes are supported when built with the pprof tag:
//
//   - allocs:    Memory allocation profiling (all allocations)
//   - block:     Block (synchronization) profiling
//   - clock:     Wall-clock profiling
//   - cpu:       CPU profiling
//   - goroutine: Goroutine profiling
//   - heap:      Heap memory profiling (live allocations)
//   - mem:       General memory profiling
//   - mutex:     Mutex contention profiling
//   - thread:    Thread creation profiling
//   - trace:     Execution trace profiling
//
// Use [Modes] to retrieve the list of supported modes programmatically.
//
// # Using File-Based Profiling
//
// File-based profiling writes profiling data to disk for later analysis. The
// profiler is configured using the [Profiler] type and started with
// [Profiler.Start]:
//
//	p := profile.Profiler{
//	    Mode:  "cpu",
//	    Path:  "/tmp/profiles",
//	    Quiet: false,
//	}
//	ctrl := p.Start()
//	defer ctrl.Stop()
//
//	// Application code runs here with profiling enabled
//
// Profile files are written to the specified directory with names matching the
// profiling mode (e.g., cpu.pprof, mem.pprof).
//
// # Command-Line Usage
//
// The aenv command supports profiling through command-line flags when built
// with the pprof tag:
//
//	# Enable CPU profiling (writes to default cache directory)
//	./aenv -pprof-mode cpu
//
//	# Enable heap profiling with custom output directory
//	./aenv -pprof-mode heap -pprof-dir ./profiles
//
//	# List available profiling modes
//	./aenv -h
//
// The default output directory is:
//
//	$XDG_CACHE_HOME/aenv/pprof   (Linux/Unix)
//	~/Library/Caches/aenv/pprof  (macOS)
//	%LocalAppData%\aenv\pprof    (Windows)
//
// # Analyzing Profile Data
//
// ## Interactive Command-Line Analysis
//
// Use the go tool pprof command to analyze profile data interactively:
//
//	# Analyze a CPU profile
//	go tool pprof /tmp/profiles/cpu.pprof
//
//	# Analyze with the original binary for symbol resolution
//	go tool pprof ./aenv /tmp/profiles/cpu.pprof
//
// Common interactive commands:
//
//	(pprof) top           # Show top functions by resource usage
//	(pprof) top10         # Show top 10 functions
//	(pprof) list main     # Show source code for functions matching "main"
//	(pprof) web           # Open graph visualization in browser
//	(pprof) pdf           # Generate PDF call graph
//	(pprof) help          # Show all available commands
//
// ## Web-Based Analysis
//
// Launch an interactive web UI for visual analysis:
//
//	# Open web UI on default port (random)
//	go tool pprof -http=: /tmp/profiles/cpu.pprof
//
//	# Open web UI on specific port
//	go tool pprof -http=localhost:8080 /tmp/profiles/cpu.pprof
//
// The web interface provides:
//   - Flame graphs for visualizing call stacks and hot paths
//   - Source code view with inline performance annotations
//   - Graph view showing call relationships and resource flow
//   - Top functions ranked by CPU time, memory, or other metrics
//   - Diff mode for comparing two profiles
//
// ## Comparing Profiles
//
// Compare two profiles to identify performance changes:
//
//	# Command-line diff
//	go tool pprof -base=old.pprof new.pprof
//
//	# Web UI with diff
//	go tool pprof -http=: -base=old.pprof new.pprof
//
// # HTTP-Based Profiling (net/http/pprof)
//
// When built with the pprof tag, this package imports [net/http/pprof], which
// registers HTTP handlers for runtime profiling at /debug/pprof/.
//
// To use HTTP profiling, your application must start an HTTP server:
//
//	import _ "net/http/pprof"
//
//	go func() {
//	    log.Println(http.ListenAndServe("localhost:6060", nil))
//	}()
//
// Common HTTP endpoints:
//
//	http://localhost:6060/debug/pprof/           # Index page
//	http://localhost:6060/debug/pprof/profile    # 30-second CPU profile
//	http://localhost:6060/debug/pprof/heap       # Heap profile
//	http://localhost:6060/debug/pprof/goroutine  # Goroutine profile
//	http://localhost:6060/debug/pprof/block      # Block profile
//	http://localhost:6060/debug/pprof/mutex      # Mutex profile
//	http://localhost:6060/debug/pprof/trace      # Execution trace (5 seconds)
//
// Retrieve and analyze HTTP profiles:
//
//	# Collect and analyze CPU profile (30 seconds)
//	go tool pprof http://localhost:6060/debug/pprof/profile
//
//	# Collect and analyze heap profile
//	go tool pprof http://localhost:6060/debug/pprof/heap
//
//	# Open web UI for live heap profiling
//	go tool pprof -http=:8080 http://localhost:6060/debug/pprof/heap
//
//	# Save profile to file for later analysis
//	curl http://localhost:6060/debug/pprof/heap > heap.pprof
//
// Query parameters for HTTP endpoints:
//
//	?seconds=60      # Duration for CPU/trace profiles (default: 30)
//	?debug=1         # Human-readable text format
//	?debug=2         # Extended text format with full goroutine stacks
//	?gc=1            # Run GC before heap profile
//
// Examples:
//
//	# 60-second CPU profile
//	curl http://localhost:6060/debug/pprof/profile?seconds=60 > cpu.pprof
//
//	# Heap profile with GC
//	curl http://localhost:6060/debug/pprof/heap?gc=1 > heap.pprof
//
//	# Human-readable goroutine dump
//	curl http://localhost:6060/debug/pprof/goroutine?debug=2
//
// # Performance Overhead
//
//   - CPU profiling: ~5% overhead
//   - Heap profiling: minimal overhead (sampled)
//   - Block profiling: can add significant overhead if rate is too high
//   - Mutex profiling: can add significant overhead if rate is too high
//   - Trace profiling: high overhead, use for short durations only
//
// Adjust sampling rates using [runtime.SetBlockProfileRate],
// [runtime.SetMutexProfileFraction], and [runtime.MemProfileRate].
package profile

// Tag is the build tag required to enable pprof profiling.
const Tag = `pprof`
