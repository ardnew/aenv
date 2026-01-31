//go:build !pprof

package cli

import (
	"context"

	"github.com/alecthomas/kong"
)

// pprof is empty when built without pprof tag.
type pprofConfig struct{}

func (pprofConfig) vars() kong.Vars { return kong.Vars{} }

func (pprofConfig) group() kong.Group { return kong.Group{} }

// start is a no-op when built without pprof tag.
func (pprofConfig) start(context.Context) (stop func()) { return func() {} }
