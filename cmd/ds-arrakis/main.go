// Command ds-arrakis bootstraps a Dune Awakening private dedicated server
// host: installs the Kubernetes distribution, fetches the Steam depot, and
// applies the Funcom operator CRDs.
//
// All application logic lives in internal/pkg/dsarrakis/cli. This file is
// wiring only: context, signal handling, dispatch.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.muehmer.eu/dapdsm/internal/pkg/dsarrakis/cli"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ds-arrakis: %s\n", err)
		if errors.Is(err, cli.ErrUsage) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr)
}
