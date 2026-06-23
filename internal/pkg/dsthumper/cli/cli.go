// Package cli implements the ds-thumper command-line interface.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// ErrUsage is returned for missing/unknown subcommands. main maps it to exit 2.
var ErrUsage = errors.New("usage error")

// Run dispatches args[0] to the matching subcommand.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return ErrUsage
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return nil
	case "version", "-v", "--version":
		fmt.Fprintln(stdout, "ds-thumper (dapdsm)")
		return nil
	case "init":
		return initCmd(ctx, args[1:], stdin, stdout, stderr)
	case "rollout":
		return rolloutCmd(ctx, args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown subcommand %q (try \"ds-thumper help\"): %w", args[0], ErrUsage)
	}
}

const usage = `ds-thumper — dapdsm suite config wizard + workstation->VM rollout.

Usage:
  ds-thumper <command> [arguments]

Commands:
  init <host>     Ensure the VM age identity, then interactively configure the
                  target (binaries, sealed secrets). Writes the workstation
                  config under $XDG_CONFIG_HOME/dapdsm. Idempotent.
  rollout <host>  Build + push the configured binaries, deliver + unseal the
                  configured secrets, and sync etc/ to the target.
  version         Print build identity.
  help            Print this message.

Requires the 'age' binary on the workstation; rollout ensures it on the VM.
`
