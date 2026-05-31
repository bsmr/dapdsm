package command

import (
	"context"
	"flag"
	"fmt"
	"io"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/lifecycle"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/schedule"
)

// shutdownCmd schedules, cancels, or queries a shutdown countdown on a host.
// Sub-commands: schedule, cancel, status.
func shutdownCmd(ctx context.Context, c *core.Core, args []string, stdout, stderr io.Writer) error {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: dunemgr shutdown <host> <schedule|cancel|status> [flags]")
		return fmt.Errorf("shutdown: usage: %w", ErrUsage)
	}
	host, sub, rest := args[0], args[1], args[2:]
	mgr := c.Schedule
	switch sub {
	case "schedule":
		fs := flag.NewFlagSet("schedule", flag.ContinueOnError)
		fs.SetOutput(stderr)
		action := fs.String("action", "stop", "lifecycle verb at deadline (stop|restart)")
		kind := fs.String("kind", "Restart", "Funcom ShutdownType (Restart|Maintenance|Update)")
		lead := fs.Int("lead", 600, "seconds from now until shutdown")
		if err := fs.Parse(rest); err != nil {
			return err
		}
		act, err := lifecycle.ValidateAction(*action)
		if err != nil {
			return err
		}
		if err := mgr.Schedule(ctx, "cli", host, schedule.Request{
			Kind: *kind, LeadSecs: *lead, Action: act,
			ShutdownDurationS: 30, BroadcastFrequency: 60, BroadcastDuration: 10,
		}); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "scheduled %s on %s in %ds\n", act, host, *lead)
		return nil
	case "cancel":
		if err := mgr.Cancel(ctx, "cli", host); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "cancelled countdown on %s\n", host)
		return nil
	case "status":
		rec, err := c.Store.GetSchedule(host)
		if err != nil {
			fmt.Fprintf(stdout, "no pending shutdown on %s\n", host)
			return nil
		}
		fmt.Fprintf(stdout, "pending %s (%s) at unix %d on %s\n", rec.Action, rec.Kind, rec.AtUnix, host)
		return nil
	default:
		fmt.Fprintf(stderr, "unknown shutdown subcommand %q (want schedule|cancel|status)\n", sub)
		return fmt.Errorf("shutdown: unknown sub %q: %w", sub, ErrUsage)
	}
}
