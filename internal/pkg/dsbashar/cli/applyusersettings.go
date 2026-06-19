package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
)

type applyUserSettingsDeps struct {
	runVendor vendorRunner
}

func defaultApplyUserSettingsDeps() applyUserSettingsDeps {
	return applyUserSettingsDeps{runVendor: execVendor}
}

func applyUserSettingsCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return applyUserSettings(ctx, args, stdout, stderr, defaultApplyUserSettingsDeps())
}

func applyUserSettings(ctx context.Context, args []string, stdout, stderr io.Writer, deps applyUserSettingsDeps) error {
	fs := flag.NewFlagSet("apply-user-settings", flag.ContinueOnError)
	fs.SetOutput(stderr)
	bin := fs.String("bg-binary", DefaultBattlegroupBin, "Path to Funcom's battlegroup wrapper")
	restart := fs.Bool("restart", false, "Also run 'battlegroup restart' after the apply")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("apply-user-settings: %w: %w", ErrUsage, err)
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "apply-user-settings: unexpected positional argument(s): %v\n", fs.Args())
		return ErrUsage
	}
	if err := deps.runVendor(ctx, *bin, "apply-default-usersettings", stdout, stderr); err != nil {
		return err
	}
	if *restart {
		if err := deps.runVendor(ctx, *bin, "restart", stdout, stderr); err != nil {
			return err
		}
	}
	return nil
}
