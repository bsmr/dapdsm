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

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/command"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
)

// ErrUsage is returned for an unknown or malformed invocation. main maps it
// to exit code 2. It aliases command.ErrUsage so dispatcher errors compare
// equal under errors.Is.
var ErrUsage = command.ErrUsage

// Run dispatches args[0]. With no args it starts the server.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	_ = stdin // reserved for future subcommands that read stdin
	if len(args) == 0 {
		return serveCmd(ctx, nil, stdout, stderr)
	}
	switch args[0] {
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	case "serve":
		return serveCmd(ctx, args[1:], stdout, stderr)
	case "--print-token":
		return serveCmd(ctx, []string{"--print-token"}, stdout, stderr)
	case "version", "-v", "--version":
		return versionCmd(ctx, args[1:], stdout, stderr)
	case "regen-token":
		return regenTokenCmd(ctx, args[1:], stdout, stderr)
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

func printUsage(w io.Writer) { fmt.Fprint(w, usage) }

const usage = `dunemgr — operate Dune Awakening private dedicated servers.

Usage:
  dunemgr [command] [arguments]

With no command, dunemgr starts the local web UI + status poller.

Commands:
  (none)              Start the web UI on the configured bind address.
  help                Print this message.
  version             Print build identity.
  regen-token         Rotate the web UI bearer token.
  host                Manage the host pool (list|add|rm|probe).
  lifecycle           Drive a BattleGroup lifecycle verb (start|stop|restart|update).
  backup              Create / list / restore BattleGroup DB backups.
  broadcast           Publish an in-game notice or shutdown announcement.
  db                  Run a read-only DB query (exec|columns|slow).
  shutdown            Schedule / cancel / inspect a shutdown countdown.
  tui                 Launch the full-screen terminal UI (status + command bar).

The web UI token is stored under the config dir; see --print-token on start.
`
