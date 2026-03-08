package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ardnew/aenv/cli"
)

func main() {
	err := cli.Run(context.Background(), os.Exit, os.Args[1:]...)
	if err != nil {
		if werr, ok := err.(interface{ Unwrap() error }); ok {
			err = werr.Unwrap()
		}

		fmt.Fprintln(os.Stderr, "error:", err)

		os.Exit(1)
	}
}
