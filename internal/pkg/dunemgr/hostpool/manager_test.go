package hostpool

import (
	"context"
	"path/filepath"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

func newTestManager(t *testing.T, runner ssh.Runner) (*Manager, *store.Store) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	m := &Manager{
		Store: s,
		SSH:   &ssh.Client{Runner: runner},
	}
	return m, s
}

func TestRegisterHappyPath(t *testing.T) {
	m, s := newTestManager(t, &fakeSSH{out: ssh.Result{Stdout: sampleKubeconfig}})
	if err := m.Register(context.Background(), "vm-a", "vm-a"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, err := s.GetHost("vm-a")
	if err != nil {
		t.Fatalf("GetHost: %v", err)
	}
	if got.FQDN != "vm-a.example.org" {
		t.Errorf("FQDN = %q, want vm-a.example.org", got.FQDN)
	}
	if got.K3sCABase64 != "TEFTRTY0Q0FEQVRB" {
		t.Errorf("CA b64 = %q, want TEFTRTY0Q0FEQVRB", got.K3sCABase64)
	}
	if got.SSHAlias != "vm-a" {
		t.Errorf("SSHAlias = %q, want vm-a", got.SSHAlias)
	}
}

func TestRegisterRejectsInvalidName(t *testing.T) {
	m, _ := newTestManager(t, &fakeSSH{out: ssh.Result{Stdout: sampleKubeconfig}})
	if err := m.Register(context.Background(), "-evil", "vm-a"); err == nil {
		t.Errorf("Register with invalid name: want error, got nil")
	}
}

func TestRegisterPropagatesSSHFailure(t *testing.T) {
	m, _ := newTestManager(t, &fakeSSH{err: ssh.ErrRemoteFailure})
	if err := m.Register(context.Background(), "vm-a", "vm-a"); err == nil {
		t.Errorf("Register with ssh failure: want error, got nil")
	}
}

func TestListAndDelete(t *testing.T) {
	m, _ := newTestManager(t, &fakeSSH{out: ssh.Result{Stdout: sampleKubeconfig}})
	if err := m.Register(context.Background(), "vm-a", "vm-a"); err != nil {
		t.Fatal(err)
	}
	if err := m.Register(context.Background(), "vm-b", "vm-b"); err != nil {
		t.Fatal(err)
	}
	all, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("len = %d, want 2", len(all))
	}
	if err := m.Delete("vm-a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	all, _ = m.List()
	if len(all) != 1 || all[0].Name != "vm-b" {
		t.Errorf("after delete: %+v", all)
	}
}
