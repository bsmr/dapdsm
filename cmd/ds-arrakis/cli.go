package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// ErrUsage is returned when the caller supplies an unrecognised or missing
// subcommand. main maps this to exit code 2.
var ErrUsage = errors.New("usage")

// dispatch routes args[0] to the appropriate subcommand handler.
// Unknown or missing subcommands return ErrUsage.
func dispatch(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: ds-arrakis <subcommand> [flags]")
		fmt.Fprintln(stderr, "subcommands: host, deploy, cluster, doctor, prepare-host")
		return ErrUsage
	}
	switch args[0] {
	case "host":
		return hostCmd(ctx, args[1:], stdout, stderr)
	case "deploy":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "usage: ds-arrakis deploy <host> [host-flags...]")
			return ErrUsage
		}
		return deploy(ctx, ssh.NewClient(), goBuilder{}, args[1], args[2:], stdout)
	case "cluster":
		return clusterCmd(ctx, ssh.NewClient(), args[1:], stdout, stderr)
	case "doctor":
		return doctorCmd(ctx, realRunner, args[1:], stdout, stderr)
	case "prepare-host":
		return prepareCmd(ctx, realRunner, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "ds-arrakis: unknown subcommand %q\n", args[0])
		fmt.Fprintln(stderr, "usage: ds-arrakis <subcommand> [flags]")
		fmt.Fprintln(stderr, "subcommands: host, deploy, cluster, doctor, prepare-host")
		return ErrUsage
	}
}
