package probe

import (
	"context"
	"strings"
	"time"

	"go.muehmer.eu/dapdsm/internal/pkg/battlegroup"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/kube"
)

// probeStatus resolves the BattleGroup namespace, fetches the CR, and
// parses its observed status. The runner is injected so callers (and
// tests) control transport.
func probeStatus(ctx context.Context, r kube.Runner) (battlegroup.Status, error) {
	ns, err := kube.FindBattleGroupNamespace(ctx, r)
	if err != nil {
		return battlegroup.Status{}, err
	}
	bg := kube.BattleGroupName(ns)
	cr, err := r.Get(ctx, "battlegroup", bg, "-n", ns, "-o", "json")
	if err != nil {
		return battlegroup.Status{}, err
	}
	return battlegroup.ParseStatus(cr)
}

// snapshotFromStatus maps a parsed Status into the cached snapshot.
// BGState is the uppercased serverGroupPhase (or UNKNOWN when empty);
// PodReady/PodTotal count ready/total servers.
func snapshotFromStatus(host string, st battlegroup.Status, probedAt time.Time) store.StatusSnapshot {
	state := strings.ToUpper(st.ServerGroupPhase)
	if state == "" {
		state = "UNKNOWN"
	}
	ready := 0
	for _, s := range st.Servers {
		if s.Ready {
			ready++
		}
	}
	return store.StatusSnapshot{
		Host:     host,
		ProbedAt: probedAt,
		BGState:  state,
		PodReady: ready,
		PodTotal: len(st.Servers),
		Detail:   st,
	}
}
