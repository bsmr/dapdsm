// Package bgorchestrator composes battlegroup lifecycle primitives into
// status-gated operations (the upgrade cycle). It depends only on the
// battlegroup domain package and a kube.Getter/Runner; it has no CLI
// knowledge so any tool can reuse it.
package bgorchestrator

import (
	"context"
	"fmt"
	"time"

	"go.muehmer.eu/dapdsm/pkg/domain/battlegroup"
	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

// Predicate is a named gate over an observed BattleGroup status.
type Predicate struct {
	Name string
	OK   func(battlegroup.Status) bool
}

// Stopped holds when no server reports Ready (server group drained).
//
// LIVE-VERIFY (A3): refine against the real .status phase strings once
// observed on dune-01 (e.g. ServerGroupPhase == "Stopped").
var Stopped = Predicate{
	Name: "stopped",
	OK: func(s battlegroup.Status) bool {
		for _, srv := range s.Servers {
			if srv.Ready {
				return false
			}
		}
		return true
	},
}

// Ready holds when at least one server exists and all report Ready.
var Ready = Predicate{
	Name: "ready",
	OK: func(s battlegroup.Status) bool {
		if len(s.Servers) == 0 {
			return false
		}
		for _, srv := range s.Servers {
			if !srv.Ready {
				return false
			}
		}
		return true
	},
}

// WaitForPhase polls the BattleGroup CR every poll interval until pred holds,
// the timeout elapses (error names the last observed phase), or ctx is
// cancelled. An initial check runs before the first tick.
func WaitForPhase(ctx context.Context, g kube.Getter, ns, bg string, pred Predicate, poll, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var last battlegroup.Status
	check := func() (bool, error) {
		cr, err := g.Get(ctx, "battlegroup", bg, "-n", ns, "-o", "json")
		if err != nil {
			return false, err
		}
		st, err := battlegroup.ParseStatus(cr)
		if err != nil {
			return false, err
		}
		last = st
		return pred.OK(st), nil
	}

	if ok, err := check(); err != nil || ok {
		return err
	}
	t := time.NewTicker(poll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for %s on %s/%s: %w (last serverGroupPhase=%q)",
				pred.Name, ns, bg, ctx.Err(), last.ServerGroupPhase)
		case <-t.C:
			if ok, err := check(); err != nil || ok {
				return err
			}
		}
	}
}
