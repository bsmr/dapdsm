package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
)

type updateDeps struct {
	runVendor vendorRunner
}

func defaultUpdateDeps() updateDeps {
	return updateDeps{runVendor: execVendor}
}

func updateCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return runUpdate(ctx, args, stdout, stderr, defaultUpdateDeps())
}

func runUpdate(ctx context.Context, args []string, stdout, stderr io.Writer, deps updateDeps) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(stderr)
	bin := fs.String("bg-binary", DefaultBattlegroupBin, "Path to Funcom's battlegroup wrapper")
	noRestart := fs.Bool("no-restart", false, "Skip the final 'battlegroup restart'")
	fromDownloads := fs.Bool("from-downloads", false, "Use 'update-from-downloads' (apply the already-downloaded Steam version) instead of 'update' (download + apply)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("update: %w: %w", ErrUsage, err)
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "update: unexpected positional argument(s): %v\n", fs.Args())
		return ErrUsage
	}
	updateAction := "update"
	if *fromDownloads {
		updateAction = "update-from-downloads"
	}
	if err := deps.runVendor(ctx, *bin, updateAction, stdout, stderr); err != nil {
		return err
	}
	if err := deps.runVendor(ctx, *bin, "apply-default-usersettings", stdout, stderr); err != nil {
		return err
	}
	if !*noRestart {
		if err := deps.runVendor(ctx, *bin, "restart", stdout, stderr); err != nil {
			return err
		}
	}
	return nil
}
