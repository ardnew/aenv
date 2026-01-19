//go:build !pprof

package cli

import (
	"context"

	"github.com/alecthomas/kong"
)

// pprof is empty when built without pprof tag.
type pprof struct{}


func (pprof) vars() kong.Vars { return kong.Vars{} }

func (pprof) group() kong.Group { return kong.Group{} }

// startProfiling is a no-op when built without pprof tag.
func (pprof) start(context.Context) (stop func()) { return func() {} }
