package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/ardnew/envcomp/cmd/envcomp/cli"
	"github.com/ardnew/envcomp/pkg/log"
)

func main() {
	err := cli.Run(context.Background(), os.Exit, os.Args[1:]...)
	if err != nil {
		log.Error("run failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
