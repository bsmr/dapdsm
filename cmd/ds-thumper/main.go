// Command ds-thumper is the dapdsm suite config wizard + workstation->VM rollout tool.
//
// All application logic lives in internal/pkg/dsthumper/cli. This file is
// wiring only: context, signal handling, dispatch.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.muehmer.eu/dapdsm/internal/pkg/dsthumper/cli"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ds-thumper: %s\n", err)
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
