package broadcast

import (
	"context"
	"fmt"
	"time"
)

// Default countdown broadcast tuning. Overridable per call (zero → default).
// Later configurable via ds-thumper.
const (
	DefaultShutdownDurationS  = 30 // expected shutdown length shown to clients
	DefaultBroadcastFrequency = 30 // client re-shows the banner this often (s)
	DefaultBroadcastDuration  = 10 // each banner visible for this long (s)
)

// AnnounceParams configures a scheduled-shutdown countdown.
type AnnounceParams struct {
	Operator string
	Host     string // audit label; also SSH alias when Exec is *ssh.Client
	Kind     string // "Restart" | "Maintenance" | "Update"
	Delay    time.Duration

	// Now is the unix base time; 0 → time.Now().Unix(). Injected for tests.
	Now int64
	// Optional tuning; zero values fall back to the Default* constants.
	ShutdownDurationS  int
	BroadcastFrequency int
	BroadcastDuration  int
	// Wait waits d (ctx-aware); nil → a real ctx-aware timer. Injected for tests.
	Wait func(ctx context.Context, d time.Duration) error
}

func realWait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Announce publishes a shutdown countdown, waits Delay, then runs action.
// If the wait is cancelled or action fails, it best-effort publishes a cancel
// broadcast and returns the originating error (action is not run on cancel).
func (r *Runner) Announce(ctx context.Context, p AnnounceParams, action func(context.Context) error) error {
	now := p.Now
	if now == 0 {
		now = time.Now().Unix()
	}
	dur := p.ShutdownDurationS
	if dur == 0 {
		dur = DefaultShutdownDurationS
	}
	freq := p.BroadcastFrequency
	if freq == 0 {
		freq = DefaultBroadcastFrequency
	}
	bdur := p.BroadcastDuration
	if bdur == 0 {
		bdur = DefaultBroadcastDuration
	}
	wait := p.Wait
	if wait == nil {
		wait = realWait
	}

	if _, err := r.PublishShutdownAnnounce(ctx, p.Operator, p.Host, ShutdownAnnounce{
		Kind:               p.Kind,
		AtUnix:             now + int64(p.Delay.Seconds()),
		NowUnix:            now,
		ShutdownDurationS:  dur,
		BroadcastFrequency: freq,
		BroadcastDuration:  bdur,
	}); err != nil {
		return fmt.Errorf("announce: publish shutdown: %w", err)
	}

	if err := wait(ctx, p.Delay); err != nil {
		_, _ = r.PublishShutdownCancel(ctx, p.Operator, p.Host) // best-effort
		return err
	}

	if err := action(ctx); err != nil {
		_, _ = r.PublishShutdownCancel(ctx, p.Operator, p.Host) // best-effort
		return err
	}
	return nil
}
