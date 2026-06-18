// Package command — whisper subcommand: a private in-game chat message.
package command

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/pkg/domain/dbquery"
	"go.muehmer.eu/dapdsm/pkg/domain/grant"
	"go.muehmer.eu/dapdsm/pkg/domain/mq"
)

// whisperCmd sends a private in-game chat message to one player over MQ. It
// pre-checks the player is online (unless --force) and never records the
// message text in the audit log.
func whisperCmd(ctx context.Context, c *core.Core, args []string, stdout, stderr io.Writer) error {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: dunemgr whisper <host> <name|fls> <message> [--from <name>] [--force] [--as <GM|Server>] [--id]")
		return fmt.Errorf("whisper: usage: %w", ErrUsage)
	}
	host, fls := args[0], args[1]
	fs := flag.NewFlagSet("whisper", flag.ContinueOnError)
	fs.SetOutput(stderr)
	from := fs.String("from", "Server", "spoofed author name shown to the player")
	force := fs.Bool("force", false, "send even if the player appears offline")
	as := fs.String("as", "", "send as a reserved persona (GM|Server) instead of a spoofed --from name")
	id := fs.Bool("id", false, "treat <player> as a raw FLS id, skip name resolution")
	msgParts, flagsTok := splitMessageAndFlags(args[2:])
	if err := fs.Parse(flagsTok); err != nil {
		return err
	}
	message := strings.Join(msgParts, " ")
	if strings.TrimSpace(message) == "" {
		fmt.Fprintln(stderr, "whisper: empty message")
		return fmt.Errorf("whisper: empty message: %w", ErrUsage)
	}

	// Detect whether --from was explicitly provided (it has a non-empty default,
	// so we cannot rely on its value alone).
	explicitFrom := false
	for _, a := range flagsTok {
		if a == "--from" || strings.HasPrefix(a, "--from=") {
			explicitFrom = true
		}
	}
	if *as != "" {
		if explicitFrom {
			fmt.Fprintln(stderr, "whisper: --as and --from are mutually exclusive")
			return fmt.Errorf("whisper: --as/--from conflict: %w", ErrUsage)
		}
		if grant.PersonaHexID(*as) == "" {
			fmt.Fprintf(stderr, "whisper: unknown persona %q (want GM|Server)\n", *as)
			return fmt.Errorf("whisper: unknown persona: %w", ErrUsage)
		}
	}

	dbr := &dbquery.Runner{SSH: c.SSH, Store: c.Store}
	var err error
	fls, err = resolvePlayerArg(ctx, dbr, host, fls, *id, stderr)
	if err != nil {
		return err
	}
	var toName string
	toName, err = dbr.CharacterName(ctx, host, fls)
	if err != nil {
		return err
	}
	found := toName != ""
	var offline bool
	offline, err = dbr.IsPlayerOffline(ctx, host, fls)
	if err != nil {
		return err
	}
	online := found && !offline
	if !online && !*force {
		if !found {
			fmt.Fprintf(stderr, "no player with fls %s on %s; whispers only reach online players. Re-run with --force to send anyway.\n", fls, host)
		} else {
			fmt.Fprintf(stderr, "player %s is offline; whispers only reach online players. Re-run with --force to send anyway.\n", fls)
		}
		return fmt.Errorf("whisper: target not online (use --force): %w", ErrUsage)
	}

	pub := &mq.Publisher{SSH: c.SSH, Store: c.Store}
	if *as != "" {
		p := &grant.Persona{DB: &dbquery.Runner{SSH: c.SSH, Store: c.Store}, Store: c.Store}
		if err := p.Seed(ctx, "cli", host, *as); err != nil {
			return err
		}
		res, err := pub.PublishWhisper(ctx, "cli", host, fls, toName, "", grant.PersonaHexID(*as), grant.PersonaFuncomID(*as), message)
		if err != nil {
			return err
		}
		if !res.OK {
			fmt.Fprintf(stderr, "whisper not confirmed:\n%s\n", res.RawOutput)
			return fmt.Errorf("whisper: publish not confirmed")
		}
		fmt.Fprintln(stdout, "whisper sent")
		return nil
	}
	res, err := pub.PublishWhisper(ctx, "cli", host, fls, toName, *from, "", "", message)
	if err != nil {
		return err
	}
	if !res.OK {
		fmt.Fprintf(stderr, "whisper not confirmed:\n%s\n", res.RawOutput)
		return fmt.Errorf("whisper: publish not confirmed")
	}
	fmt.Fprintln(stdout, "whisper sent")
	return nil
}

// splitMessageAndFlags returns the leading non-flag tokens (the message) and
// the remaining tokens starting at the first "--flag".
func splitMessageAndFlags(args []string) (msg []string, flags []string) {
	for i, a := range args {
		if strings.HasPrefix(a, "--") {
			return args[:i], args[i:]
		}
	}
	return args, nil
}
