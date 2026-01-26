//go:build !pprof

package profile

// Modes returns nil when the pprof build tag is not set.
func Modes() []string { return nil }

type control = ignore

// start returns a no-op controller when the pprof build tag is not set.
func start(string, string, bool) interface{ Stop() } { return control{} }
