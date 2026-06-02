// Package command — whisper subcommand: a private in-game chat message.
package command

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/dbquery"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/mq"
)

// whisperCmd sends a private in-game chat message to one player over MQ. It
// pre-checks the player is online (unless --force) and never records the
// message text in the audit log.
func whisperCmd(ctx context.Context, c *core.Core, args []string, stdout, stderr io.Writer) error {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: dunemgr whisper <host> <fls-id> <message> [--from <name>] [--force]")
		return fmt.Errorf("whisper: usage: %w", ErrUsage)
	}
	host, fls := args[0], args[1]
	fs := flag.NewFlagSet("whisper", flag.ContinueOnError)
	fs.SetOutput(stderr)
	from := fs.String("from", "Server", "spoofed author name shown to the player")
	force := fs.Bool("force", false, "send even if the player appears offline")
	msgParts, rest := splitMessageAndFlags(args[2:])
	if err := fs.Parse(rest); err != nil {
		return err
	}
	message := strings.Join(msgParts, " ")
	if strings.TrimSpace(message) == "" {
		fmt.Fprintln(stderr, "whisper: empty message")
		return fmt.Errorf("whisper: empty message: %w", ErrUsage)
	}

	dbr := &dbquery.Runner{SSH: c.SSH, Store: c.Store}
	toName, err := dbr.CharacterName(ctx, host, fls)
	if err != nil {
		return err
	}
	found := toName != ""
	offline, err := dbr.IsPlayerOffline(ctx, host, fls)
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
	res, err := pub.PublishWhisper(ctx, "cli", host, fls, toName, *from, "", message)
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
