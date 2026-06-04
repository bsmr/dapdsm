// Package command is the unified dunemgr command dispatcher. Each handler
// receives an already-open *core.Core (never opening its own bbolt store),
// so the CLI entrypoint and the TUI command bar can both feed verbs into one
// dispatch table — a single source of truth for argument parsing/validation.
package command

import (
	"context"
	"errors"
	"fmt"
	"io"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
)

// ErrUsage is returned for an unknown or malformed invocation. The CLI maps
// it to exit code 2 (via a cli.ErrUsage alias).
var ErrUsage = errors.New("usage error")

// Handler runs one verb against the shared core. argv is the full argument
// tail for the verb (argv[0] is the verb itself).
type Handler func(ctx context.Context, c *core.Core, argv []string, stdout, stderr io.Writer) error

// table maps a verb to its handler. Handlers register here as they are
// migrated; host-targeting verbs only (version/regen-token stay in cli).
var table = map[string]Handler{
	"admin":     adminCmd,
	"avatar":    avatarCmd,
	"backup":    backupCmd,
	"broadcast": broadcastCmd,
	"db":        dbCmd,
	"give":      giveCmd,
	"host":      hostCmd,
	"item":      itemCmd,
	"ini":       iniCmd,
	"lifecycle": lifecycleCmd,
	"player":    playerCmd,
	"shutdown":  shutdownCmd,
	"stats":     statsCmd,
	"whisper":   whisperCmd,
}

// Dispatch looks up argv[0] and invokes its handler with the shared core.
func Dispatch(ctx context.Context, c *core.Core, argv []string, stdout, stderr io.Writer) error {
	if len(argv) == 0 {
		return fmt.Errorf("no command: %w", ErrUsage)
	}
	h, ok := table[argv[0]]
	if !ok {
		return fmt.Errorf("unknown subcommand %q (try \"dunemgr help\"): %w", argv[0], ErrUsage)
	}
	return h(ctx, c, argv[1:], stdout, stderr)
}

// Known reports whether verb has a registered handler. Callers use it to
// avoid building the shared core (which opens the bbolt store) for a verb
// that Dispatch would reject as ErrUsage anyway.
func Known(verb string) bool {
	_, ok := table[verb]
	return ok
}
