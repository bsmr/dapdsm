// Package command — broadcast subcommand.
package command

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/broadcast"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
)

// broadcastCmd publishes an in-game notice or shutdown announcement
// to the BattleGroup running on the named host.
func broadcastCmd(ctx context.Context, c *core.Core, args []string, stdout, stderr io.Writer) error {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: dunemgr broadcast <host> <notice|shutdown|shutdown-cancel> [flags]")
		return fmt.Errorf("broadcast: usage: %w", ErrUsage)
	}
	host, kind, rest := args[0], args[1], args[2:]
	r := &broadcast.Runner{SSH: c.SSH, Store: c.Store}
	switch kind {
	case "notice":
		fs := flag.NewFlagSet("notice", flag.ContinueOnError)
		fs.SetOutput(stderr)
		title := fs.String("title", "", "banner title")
		body := fs.String("body", "", "banner body")
		dur := fs.Int("duration", 30, "banner visible duration (seconds)")
		if err := fs.Parse(rest); err != nil {
			return err
		}
		if *title == "" || *body == "" {
			fmt.Fprintln(stderr, "usage: dunemgr broadcast <host> notice --title T --body B [--duration 30]")
			return fmt.Errorf("notice: --title and --body required: %w", ErrUsage)
		}
		res, err := r.PublishNotice(ctx, "cli", host, *title, *body, *dur)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "publish ok=%v\n%s\n", res.OK, res.RawOutput)
		return nil
	case "shutdown":
		fs := flag.NewFlagSet("shutdown", flag.ContinueOnError)
		fs.SetOutput(stderr)
		k := fs.String("kind", "Restart", "ShutdownType (Restart|Maintenance|Update)")
		atIn := fs.Int("at-secs", 300, "seconds from now until shutdown")
		dur := fs.Int("duration", 30, "expected shutdown duration (seconds)")
		freq := fs.Int("freq", 60, "banner repeat frequency (seconds)")
		bdur := fs.Int("banner-duration", 10, "banner visible duration each repeat (seconds)")
		if err := fs.Parse(rest); err != nil {
			return err
		}
		now := time.Now().Unix()
		res, err := r.PublishShutdownAnnounce(ctx, "cli", host, broadcast.ShutdownAnnounce{
			Kind:               *k,
			NowUnix:            now,
			AtUnix:             now + int64(*atIn),
			ShutdownDurationS:  *dur,
			BroadcastFrequency: *freq,
			BroadcastDuration:  *bdur,
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "publish ok=%v\n%s\n", res.OK, res.RawOutput)
		return nil
	case "shutdown-cancel":
		res, err := r.PublishShutdownCancel(ctx, "cli", host)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "publish ok=%v\n%s\n", res.OK, res.RawOutput)
		return nil
	default:
		fmt.Fprintf(stderr, "unknown broadcast kind %q (want notice|shutdown|shutdown-cancel)\n", kind)
		return fmt.Errorf("broadcast: unknown kind %q: %w", kind, ErrUsage)
	}
}
