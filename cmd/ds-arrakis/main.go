// Command ds-arrakis bootstraps a Dune Awakening private dedicated server
// host: installs the Kubernetes distribution, fetches the Steam depot, and
// applies the Funcom operator CRDs.
//
// All application logic lives in pkg/domain/bootstrap and pkg/transport/*.
// This file is wiring only: context, signal handling, dispatch.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "ds-arrakis: %s\n", err)
		if errors.Is(err, ErrUsage) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	return dispatch(ctx, args, stdout, stderr)
}
