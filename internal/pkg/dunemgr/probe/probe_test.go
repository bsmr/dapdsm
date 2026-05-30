package probe

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/kube"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// withFakeGetter swaps newGetter for the duration of a test.
func withFakeGetter(t *testing.T, g kube.Getter) {
	t.Helper()
	orig := newGetter
	newGetter = func(*ssh.Client, string) kube.Getter { return g }
	t.Cleanup(func() { newGetter = orig })
}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestProbeReportsRealStatus(t *testing.T) {
	withFakeGetter(t, &kubeFake{nsOut: "default\nfuncom-seabass-sh-abc\n", crOut: crJSON})
	s := newStore(t)
	_ = s.PutHost(store.HostProfile{Name: "vm-a", SSHAlias: "vm-a"})

	snap, err := Probe(context.Background(), s, &ssh.Client{}, "vm-a")
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if snap.BGState != "RUNNING" {
		t.Errorf("BGState = %q, want RUNNING", snap.BGState)
	}
	if snap.PodReady != 1 || snap.PodTotal != 2 {
		t.Errorf("ready/total = %d/%d, want 1/2", snap.PodReady, snap.PodTotal)
	}
}

func TestProbePersistsSnapshot(t *testing.T) {
	withFakeGetter(t, &kubeFake{nsOut: "funcom-seabass-x\n", crOut: crJSON})
	s := newStore(t)
	_ = s.PutHost(store.HostProfile{Name: "vm-a", SSHAlias: "vm-a"})

	_, _ = Probe(context.Background(), s, &ssh.Client{}, "vm-a")
	cached, err := s.GetStatus("vm-a")
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if cached.BGState != "RUNNING" {
		t.Errorf("cached BGState = %q, want RUNNING", cached.BGState)
	}
}

func TestProbeKubectlErrorRecordsUnknown(t *testing.T) {
	withFakeGetter(t, &kubeFake{err: errors.New("ssh: connection refused")})
	s := newStore(t)
	_ = s.PutHost(store.HostProfile{Name: "vm-a", SSHAlias: "vm-a"})

	snap, err := Probe(context.Background(), s, &ssh.Client{}, "vm-a")
	if err != nil {
		t.Fatalf("Probe returned hard error: %v", err)
	}
	if snap.BGState != "UNKNOWN" || snap.Error == "" {
		t.Errorf("snap = %+v, want UNKNOWN + error", snap)
	}
}

func TestProbeUnregisteredHostErrors(t *testing.T) {
	withFakeGetter(t, &kubeFake{crOut: crJSON})
	s := newStore(t)
	if _, err := Probe(context.Background(), s, &ssh.Client{}, "ghost"); err == nil {
		t.Fatal("Probe on unregistered host returned nil error")
	}
}
