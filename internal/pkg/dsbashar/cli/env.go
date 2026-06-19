package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"go.muehmer.eu/dapdsm/internal/pkg/dsbashar/config"
)

func envCmd(_ context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("env", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("config", config.DefaultPath, "Path to dunectl.env")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("env: %w: %w", ErrUsage, err)
	}

	cfg, err := config.LoadFromFile(*path)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "config:            %s\n", *path)
	fmt.Fprintf(stdout, "TARGET:            %s\n", cfg.Target)
	fmt.Fprintf(stdout, "FLS_TOKEN_FILE:    %s\n", cfg.FLSTokenFile)
	if len(cfg.K3SExtraTLSSANs) > 0 {
		fmt.Fprintf(stdout, "K3S_EXTRA_TLS_SAN: %s\n", strings.Join(cfg.K3SExtraTLSSANs, " "))
	} else {
		fmt.Fprintln(stdout, "K3S_EXTRA_TLS_SAN: (none)")
	}
	fmt.Fprintf(stdout, "(derived) AppID:   %d\n", cfg.AppID())
	return nil
}
