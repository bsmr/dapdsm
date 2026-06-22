package bgorchestrator

import (
	"context"
	"fmt"
	"time"

	"go.muehmer.eu/dapdsm/pkg/domain/battlegroup"
	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

// Config tunes the upgrade cycle. Zero values get sane defaults.
type Config struct {
	Poll    time.Duration // status poll interval (default 5s)
	Timeout time.Duration // per-gate timeout (default 5m)
	OnPhase func(string)  // progress callback; nil = discard
}

func (c Config) withDefaults() Config {
	if c.Poll <= 0 {
		c.Poll = 5 * time.Second
	}
	if c.Timeout <= 0 {
		c.Timeout = 5 * time.Minute
	}
	if c.OnPhase == nil {
		c.OnPhase = func(string) {}
	}
	return c
}

// Upgrade runs the operator's stop → update → start cycle, gating on observed
// status between steps. newTag is the already-staged depot revision (staging
// is ds-arrakis' job, done before this call). Each gate has its own timeout.
func Upgrade(ctx context.Context, r kube.Runner, ns, bg, newTag string, cfg Config) error {
	cfg = cfg.withDefaults()

	cfg.OnPhase("stopping")
	if err := battlegroup.Stop(ctx, r, ns, bg); err != nil {
		return fmt.Errorf("upgrade: stop: %w", err)
	}
	if err := WaitForPhase(ctx, r, ns, bg, Stopped, cfg.Poll, cfg.Timeout); err != nil {
		return fmt.Errorf("upgrade: gate stopped: %w", err)
	}
	cfg.OnPhase("stopped")

	cfg.OnPhase("updating")
	if err := battlegroup.Update(ctx, r, ns, bg, newTag); err != nil {
		return fmt.Errorf("upgrade: update: %w", err)
	}
	cfg.OnPhase("updated")

	cfg.OnPhase("starting")
	if err := battlegroup.Start(ctx, r, ns, bg); err != nil {
		return fmt.Errorf("upgrade: start: %w", err)
	}
	if err := WaitForPhase(ctx, r, ns, bg, Ready, cfg.Poll, cfg.Timeout); err != nil {
		return fmt.Errorf("upgrade: gate ready: %w", err)
	}
	cfg.OnPhase("ready")
	return nil
}
