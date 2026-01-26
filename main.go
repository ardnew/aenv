package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/ardnew/aenv/cli"
	"github.com/ardnew/aenv/log"
)

func main() {
	err := cli.Run(context.Background(), os.Exit, os.Args[1:]...)
	if err != nil {
		log.Error(
			"run failed",
			slog.Any("error", err),
		) // slog automatically uses LogValue()
		os.Exit(1)
	}
}
