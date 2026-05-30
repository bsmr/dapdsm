// Command dunemgr operates Dune Awakening private dedicated servers via a
// local web UI and CLI subcommands.
//
// All application logic lives in internal/pkg/dunemgr/cli. This file is
// wiring only: it builds the context, signal handling, and dispatches to
// cli.Run.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/cli"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "dunemgr: %s\n", err)
		if errors.Is(err, cli.ErrUsage) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return cli.Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
}
