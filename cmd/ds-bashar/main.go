// Command ds-bashar orchestrates Dune Awakening private dedicated servers.
//
// All application logic lives in internal/pkg/dsbashar/*. This file is wiring
// only: it builds the context, signal handling, and dispatches to cli.Run.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.muehmer.eu/dapdsm/internal/pkg/dsbashar/cli"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ds-bashar: %s\n", err)
		if errors.Is(err, cli.ErrUsage) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return cli.Run(ctx, os.Args[1:], ssh.NewClient(), os.Stdin, os.Stdout, os.Stderr)
}
