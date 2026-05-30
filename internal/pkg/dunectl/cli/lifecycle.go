package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/exec"
)

// DefaultBattlegroupBin is the location of the Funcom-vendor wrapper
// that orchestrates a BattleGroup's lifecycle (stop / start / restart).
// The wrapper handles operator timing that a plain `kubectl patch` of
// spec.stop would not — so we shell out instead of duplicating it.
const DefaultBattlegroupBin = "/home/dune/.dune/bin/battlegroup"

// vendorRunner runs a single subcommand of the Funcom battlegroup binary
// and streams its output to stdout/stderr. Tests substitute their own.
type vendorRunner func(ctx context.Context, bin, action string, stdout, stderr io.Writer) error

func execVendor(ctx context.Context, bin, action string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, bin, action)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", bin, action, err)
	}
	return nil
}

type lifecycleDeps struct {
	runVendor vendorRunner
}

func defaultLifecycleDeps() lifecycleDeps {
	return lifecycleDeps{runVendor: execVendor}
}

func startCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return runLifecycle(ctx, "start", args, stdout, stderr, defaultLifecycleDeps())
}

func stopCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return runLifecycle(ctx, "stop", args, stdout, stderr, defaultLifecycleDeps())
}

func restartCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return runLifecycle(ctx, "restart", args, stdout, stderr, defaultLifecycleDeps())
}

func runLifecycle(ctx context.Context, action string, args []string, stdout, stderr io.Writer, deps lifecycleDeps) error {
	fs := flag.NewFlagSet(action, flag.ContinueOnError)
	fs.SetOutput(stderr)
	bin := fs.String("bg-binary", DefaultBattlegroupBin, "Path to Funcom's battlegroup wrapper")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%s: %w: %w", action, ErrUsage, err)
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "%s: unexpected positional argument(s): %v\n", action, fs.Args())
		return ErrUsage
	}
	return deps.runVendor(ctx, *bin, action, stdout, stderr)
}
