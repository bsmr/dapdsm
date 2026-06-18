package store

import (
	"path/filepath"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/domain/battlegroup"
)

func TestStatusSnapshot_DetailRoundTrip(t *testing.T) {
	t.Parallel()
	s, err := Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	want := StatusSnapshot{
		Host:     "vm-a",
		BGState:  "RUNNING",
		PodReady: 2,
		PodTotal: 3,
		Detail: battlegroup.Status{
			ServerGroupPhase: "Running",
			DBPhase:          "Ready",
			Servers: []battlegroup.ServerStatus{
				{Map: "Overmap", Phase: "Running", Ready: true, GamePort: 7779},
			},
		},
	}
	if err := s.PutStatus(want); err != nil {
		t.Fatalf("PutStatus: %v", err)
	}
	got, err := s.GetStatus("vm-a")
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if len(got.Detail.Servers) != 1 || got.Detail.Servers[0].Map != "Overmap" {
		t.Errorf("Detail.Servers = %+v, want one Overmap entry", got.Detail.Servers)
	}
	if got.Detail.DBPhase != "Ready" {
		t.Errorf("Detail.DBPhase = %q, want Ready", got.Detail.DBPhase)
	}
}
