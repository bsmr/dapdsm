// Package cli implements the dunemgr command-line interface.
//
// Run dispatches an argv tail to the matching subcommand. Subcommands receive
// a context plus injected stdout/stderr and never call os.Exit; they return
// errors that main converts into a process exit status. Mirrors the dunectl
// cli package.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// ErrUsage is returned for an unknown or malformed invocation. main maps it
// to exit code 2.
var ErrUsage = errors.New("usage error")

// Run dispatches args[0] to the matching subcommand. With no args it starts
// the server.
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
	case "host":
		return hostCmd(ctx, args[1:], stdout, stderr)
	case "lifecycle":
		return lifecycleCmd(ctx, args[1:], stdout, stderr)
	case "broadcast":
		return broadcastCmd(ctx, args[1:], stdout, stderr)
	case "db":
		return dbCmd(ctx, args[1:], stdout, stderr)
	case "backup":
		return backupCmd(ctx, args[1:], stdout, stderr)
	case "shutdown":
		return shutdownCmd(ctx, args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown subcommand %q (try \"dunemgr help\"): %w", args[0], ErrUsage)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, usage)
}

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

The web UI token is stored under the config dir; see --print-token on start.
`
