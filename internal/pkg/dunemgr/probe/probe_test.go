package probe

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/tunnel"
	"go.muehmer.eu/dapdsm/internal/pkg/kube"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// noopSSH satisfies the tunnel.Manager's SSH dependency without doing I/O.
type noopSSH struct{}

func (noopSSH) Run(context.Context, string, ...string) (ssh.Result, error) {
	return ssh.Result{}, nil
}
func (noopSSH) RunWithStdin(context.Context, []byte, string, ...string) (ssh.Result, error) {
	return ssh.Result{}, nil
}

// withFakeKubeRunner swaps newKubeRunner for the duration of a test.
func withFakeKubeRunner(t *testing.T, r kube.Runner) {
	t.Helper()
	orig := newKubeRunner
	newKubeRunner = func(string, io.Writer) kube.Runner { return r }
	t.Cleanup(func() { newKubeRunner = orig })
}

func TestProbeReportsRealStatus(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	withFakeKubeRunner(t, &kubeFake{
		nsOut: "default\nfuncom-seabass-sh-abc\n",
		crOut: crJSON, // from status_test.go: Running, 2 servers (1 ready)
	})
	s, _ := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	defer s.Close()
	_ = s.PutHost(store.HostProfile{Name: "vm-a", SSHAlias: "vm-a"})
	tm := &tunnel.Manager{SSH: &ssh.Client{Runner: noopSSH{}}}
	_ = tm.Connect(context.Background(), "vm-a")

	snap, err := Probe(context.Background(), s, tm, "vm-a")
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if snap.BGState != "RUNNING" {
		t.Errorf("BGState = %q, want RUNNING", snap.BGState)
	}
	if snap.PodReady != 1 || snap.PodTotal != 2 {
		t.Errorf("ready/total = %d/%d, want 1/2", snap.PodReady, snap.PodTotal)
	}
	if len(snap.Detail.Servers) != 2 {
		t.Errorf("Detail.Servers = %+v, want 2", snap.Detail.Servers)
	}
}

func TestProbePersistsSnapshot(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	withFakeKubeRunner(t, &kubeFake{nsOut: "funcom-seabass-x\n", crOut: crJSON})
	s, _ := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	defer s.Close()
	_ = s.PutHost(store.HostProfile{Name: "vm-a", SSHAlias: "vm-a"})
	tm := &tunnel.Manager{SSH: &ssh.Client{Runner: noopSSH{}}}
	_ = tm.Connect(context.Background(), "vm-a")

	_, _ = Probe(context.Background(), s, tm, "vm-a")
	cached, err := s.GetStatus("vm-a")
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if cached.Host != "vm-a" || cached.BGState != "RUNNING" {
		t.Errorf("cached = %+v, want vm-a/RUNNING", cached)
	}
}

func TestProbeNoNamespaceRecordsError(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	withFakeKubeRunner(t, &kubeFake{nsOut: "default\nkube-system\n"})
	s, _ := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	defer s.Close()
	_ = s.PutHost(store.HostProfile{Name: "vm-a", SSHAlias: "vm-a"})
	tm := &tunnel.Manager{SSH: &ssh.Client{Runner: noopSSH{}}}
	_ = tm.Connect(context.Background(), "vm-a")

	snap, err := Probe(context.Background(), s, tm, "vm-a")
	if err != nil {
		t.Fatalf("Probe returned hard error: %v", err)
	}
	if snap.BGState != "UNKNOWN" || snap.Error == "" {
		t.Errorf("snap = %+v, want UNKNOWN with Error set", snap)
	}
}
