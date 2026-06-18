// Package cli implements the dunemgr command-line entrypoint: it parses the
// top-level verb, builds the shared core once, and delegates host-targeting
// verbs to the command dispatcher. main converts returned errors into an
// exit status.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/command"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
)

// ErrUsage is returned for an unknown or malformed invocation. main maps it
// to exit code 2. It aliases command.ErrUsage so dispatcher errors compare
// equal under errors.Is.
var ErrUsage = command.ErrUsage

// Run dispatches args[0]. With no args it launches the TUI.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	_ = stdin // reserved for future subcommands that read stdin
	if len(args) == 0 {
		return tuiCmd(ctx, nil, stdout, stderr)
	}
	switch args[0] {
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	case "version", "-v", "--version":
		return versionCmd(ctx, args[1:], stdout, stderr)
	case "tui":
		return tuiCmd(ctx, args[1:], stdout, stderr)
	default:
		// An unknown verb is rejected without opening the store: route it
		// through Dispatch with a nil core, which returns ErrUsage before
		// touching the core. Known host-targeting verbs (host, lifecycle,
		// broadcast, db, backup, shutdown) get the shared core.
		if !command.Known(args[0]) {
			return command.Dispatch(ctx, nil, args, stdout, stderr)
		}
		c, err := core.Open(os.Getenv)
		if err != nil {
			return err
		}
		defer c.Close()
		return command.Dispatch(ctx, c, args, stdout, stderr)
	}
}

const usageHeader = `dunemgr — operate Dune Awakening private dedicated servers.

Usage:
  dunemgr [command] [arguments]

With no command, dunemgr launches the terminal UI (TUI).

Commands:
`

func printUsage(w io.Writer) {
	fmt.Fprint(w, usageHeader)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	// CLI-only verbs (static).
	fmt.Fprintln(tw, "  (none)\tLaunch the full-screen terminal UI (status + command bar).")
	fmt.Fprintln(tw, "  help\tPrint this message.")
	fmt.Fprintln(tw, "  version\tPrint build identity.")
	fmt.Fprintln(tw, "  tui\tLaunch the full-screen terminal UI (status + command bar).")
	// Dispatcher verbs: generated from command.Specs() so this list never goes stale.
	for _, s := range command.Specs() {
		fmt.Fprintf(tw, "  %s\t%s\n", s.Verb, s.Summary)
	}
	_ = tw.Flush()
}
