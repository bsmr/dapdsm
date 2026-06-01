package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"go.muehmer.eu/dapdsm/internal/pkg/version"
)

func versionCmd(_ context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("version: %w: %w", ErrUsage, err)
	}
	fmt.Fprintln(stdout, version.String("dunectl"))
	return nil
}
