package server

import (
	"testing"
	"time"

	"go.muehmer.eu/dapdsm/pkg/domain/battlegroup"
	"go.muehmer.eu/dapdsm/pkg/domain/store"
)

func TestBuildLifecycleView_Populated(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	snap := store.StatusSnapshot{
		Host: "vm-a", BGState: "RUNNING", PodReady: 1, PodTotal: 2,
		Detail: battlegroup.Status{
			ServerGroupPhase: "Running", DBPhase: "Ready", DirectorPhase: "Ready",
			Size: 2, StartedAt: now.Add(-90 * time.Minute),
			Servers: []battlegroup.ServerStatus{
				{Map: "Overmap", Phase: "Running", Ready: true, GamePort: 7779},
				{Map: "Survival_1", Phase: "Stopped", Ready: false, ExitReason: "clean"},
			},
		},
	}
	v := buildLifecycleView("vm-a", snap, true, now)
	if v.Waiting {
		t.Error("Waiting = true for populated snapshot")
	}
	if v.Host != "vm-a" || v.BGState != "RUNNING" || v.DBPhase != "Ready" {
		t.Errorf("header = %+v", v)
	}
	if v.Uptime != "1h30m" {
		t.Errorf("Uptime = %q, want 1h30m", v.Uptime)
	}
	if len(v.Servers) != 2 || v.Servers[0].Map != "Overmap" {
		t.Errorf("Servers = %+v", v.Servers)
	}
}

func TestBuildLifecycleView_EmptyCacheWaits(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	v := buildLifecycleView("vm-a", store.StatusSnapshot{}, false, now)
	if !v.Waiting {
		t.Error("Waiting = false for empty (never-probed) snapshot")
	}
	if v.Host != "vm-a" {
		t.Errorf("Host = %q", v.Host)
	}
}
