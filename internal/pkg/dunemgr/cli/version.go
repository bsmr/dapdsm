package cli

import (
	"context"
	"fmt"
	"io"

	"go.muehmer.eu/dapdsm/pkg/version"
)

// versionCmd prints the dunemgr build identity (shared dapdsm version package).
func versionCmd(_ context.Context, _ []string, stdout, _ io.Writer) error {
	fmt.Fprintln(stdout, version.String("dunemgr"))
	return nil
}
