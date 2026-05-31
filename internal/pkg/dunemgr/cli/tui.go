package cli

import (
	"context"
	"io"
	"os"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/tui"
)

// tuiCmd launches the full-screen terminal UI. It builds the shared core
// (which the TUI holds open for the background poller) and hands off to
// tui.Run; the bubbletea program owns the terminal until the user quits.
func tuiCmd(ctx context.Context, _ []string, _, _ io.Writer) error {
	c, err := core.Open(os.Getenv)
	if err != nil {
		return err
	}
	defer c.Close()
	return tui.Run(ctx, c)
}
