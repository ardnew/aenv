package cmd

import (
	"context"
	"log/slog"

	"github.com/ardnew/aenv/log"
)

const VersionIdentifier = "version"

type Version struct {
	Semantic bool `default:"false" help:"Print undecorated semantic version" short:"s"`
}

func (v *Version) Run(ctx context.Context) (err error) {
	ctx, cancel := context.WithCancelCause(ctx)

	defer func(err *error) { cancel(*err) }(&err)

	ktx := kongContextFrom(ctx)

	version, ok := ktx.Model.Vars()[VersionIdentifier]
	if !ok {
		return ErrMissingConfig
	}

	if v.Semantic {
		log.Raw(version)
	} else {
		log.Print(slog.String(VersionIdentifier, version))
	}

	return nil
}
