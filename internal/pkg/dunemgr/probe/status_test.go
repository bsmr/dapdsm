package probe

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.muehmer.eu/dapdsm/internal/pkg/battlegroup"
)

// kubeFake implements kube.Getter. Get switches on the resource arg:
// "ns" returns the namespace list; any other resource returns the CR JSON.
// If err is set, Get always returns that error.
type kubeFake struct {
	nsOut string
	crOut string
	err   error
}

func (f *kubeFake) Get(_ context.Context, args ...string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	if len(args) > 0 && args[0] == "ns" {
		return []byte(f.nsOut), nil
	}
	return []byte(f.crOut), nil
}

const crJSON = `{"status":{"serverGroupPhase":"Running","database":{"phase":"Ready"},
"servers":[{"partitionMap":"Overmap","phase":"Running","ready":true,"gamePort":7779},
{"partitionMap":"Survival_1","phase":"Stopped","ready":false}]}}`

func TestProbeStatus_ParsesLiveCR(t *testing.T) {
	t.Parallel()
	r := &kubeFake{nsOut: "default\nfuncom-seabass-sh-abc\n", crOut: crJSON}
	st, err := probeStatus(context.Background(), r)
	if err != nil {
		t.Fatalf("probeStatus err = %v", err)
	}
	if st.ServerGroupPhase != "Running" || len(st.Servers) != 2 {
		t.Errorf("got %+v, want Running/2 servers", st)
	}
}

func TestProbeStatus_NoNamespace(t *testing.T) {
	t.Parallel()
	r := &kubeFake{nsOut: "default\nkube-system\n"}
	_, err := probeStatus(context.Background(), r)
	if err == nil || !strings.Contains(err.Error(), "no funcom-seabass-* namespace") {
		t.Errorf("err = %v, want namespace error", err)
	}
}

func TestSnapshotFromStatus_Maps(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 29, 9, 0, 0, 0, time.UTC)
	st := battlegroup.Status{
		ServerGroupPhase: "Running",
		Servers: []battlegroup.ServerStatus{
			{Map: "A", Ready: true},
			{Map: "B", Ready: false},
			{Map: "C", Ready: true},
		},
	}
	snap := snapshotFromStatus("vm-a", st, now)
	if snap.BGState != "RUNNING" {
		t.Errorf("BGState = %q, want RUNNING (uppercased)", snap.BGState)
	}
	if snap.PodReady != 2 || snap.PodTotal != 3 {
		t.Errorf("ready/total = %d/%d, want 2/3", snap.PodReady, snap.PodTotal)
	}
	if snap.Host != "vm-a" || !snap.ProbedAt.Equal(now) {
		t.Errorf("Host/ProbedAt = %q/%v", snap.Host, snap.ProbedAt)
	}
	if len(snap.Detail.Servers) != 3 {
		t.Errorf("Detail not carried: %+v", snap.Detail)
	}
}

func TestSnapshotFromStatus_EmptyPhaseIsUnknown(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 29, 9, 0, 0, 0, time.UTC)
	snap := snapshotFromStatus("vm-a", battlegroup.Status{}, now)
	if snap.BGState != "UNKNOWN" {
		t.Errorf("BGState = %q, want UNKNOWN for empty phase", snap.BGState)
	}
}
