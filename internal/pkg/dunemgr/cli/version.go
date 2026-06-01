package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/config"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/internal/pkg/version"
)

// versionCmd prints the dunemgr build identity (shared dapdsm version package).
func versionCmd(_ context.Context, _ []string, stdout, _ io.Writer) error {
	fmt.Fprintln(stdout, version.String("dunemgr"))
	return nil
}

// regenTokenCmd rotates the bearer token in the config dir and prints the
// new value plus where it was written.
func regenTokenCmd(_ context.Context, _ []string, stdout, _ io.Writer) error {
	dir, err := core.ConfigDir(os.Getenv)
	if err != nil {
		return err
	}
	tok, err := config.RegenToken(dir)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "new token: %s\n(written to %s/token)\n", tok, dir)
	return nil
}
