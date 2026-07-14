package cli

import (
	"slices"

	"github.com/ardnew/aenv/exit"
	"github.com/ardnew/aenv/lang"
	"github.com/ardnew/aenv/log"
)

// Namespace is the environment subcommand. It takes a namespace name; extra
// args pass to the generator.
type Namespace struct {
	logFlags
	inputFlags

	// Namespace names the environment to generate.
	Namespace string `arg:""`
	// Args pass to the generator.
	Args []string `arg:"" optional:"" name:"args" help:"Namespace arguments."`

	ast lang.AST
}

// Run executes the namespace subcommand.
func (n Namespace) Run() error {
	n.Source = slices.DeleteFunc(n.Source,
		func(s string) bool { return s == "" })

	log.Debug(log.Attrs(
		"name", "namespace",
		"value", n.Namespace,
		"args", len(n.Args),
		"sources", len(n.Source),
		"handlers", len(n.Log),
		"verbose", n.Verbose,
	), "command")
	return withLogHandlers(n.logFlags, func() error {
		if err := withSources(n.Source, &n); err != nil {
			return err
		}
		log.Debug(log.Attrs("cmd", "namespace", "value", n.Namespace))
		return nil
	})
}

func (n *Namespace) Write(b []byte) (int, error) {
	nb, err := n.ast.Write(b)
	if err != nil {
		log.Debug(log.Attrs("error", err))
		return nb, withExitCode(err, exit.Data)
	}
	return nb, nil
}
