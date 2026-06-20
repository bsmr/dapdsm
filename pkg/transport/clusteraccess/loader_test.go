package clusteraccess

import (
	"context"
	"errors"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// fakeExecer records each call as a flat [host, cmd, args...] slice and returns canned stdout/err.
type fakeExecer struct {
	calls  [][]string // each entry: host, cmd, args...
	stdout string
	err    error
}

func (f *fakeExecer) Run(_ context.Context, host, cmd string, args ...string) (ssh.Result, error) {
	rec := append([]string{host, cmd}, args...)
	f.calls = append(f.calls, rec)
	return ssh.Result{Stdout: f.stdout}, f.err
}

func TestLoad(t *testing.T) {
	fe := &fakeExecer{stdout: sampleInventory}
	d, err := Load(context.Background(), fe, LoadParams{
		JumpHost:       "jump",
		KubeconfigPath: "/home/installer/.clusters/x/kubeconfig",
		InventoryPath:  "/home/installer/.clusters/x/inventory.yml",
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d.JumpHost != "jump" || d.Distro != "rke2" {
		t.Errorf("descriptor = %+v", d)
	}
	if d.Kubeconfig != "/home/installer/.clusters/x/kubeconfig" {
		t.Errorf("kubeconfig = %q", d.Kubeconfig)
	}
	if d.NodeUser != "installer" || len(d.Nodes) != 4 {
		t.Errorf("user=%q nodes=%d", d.NodeUser, len(d.Nodes))
	}
	// It must read the inventory by cat-ing it on the jumphost.
	if len(fe.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(fe.calls))
	}
	got := fe.calls[0]
	want := []string{"jump", "cat", "/home/installer/.clusters/x/inventory.yml"}
	if len(got) != len(want) {
		t.Fatalf("call = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("call = %v, want %v", got, want)
		}
	}
}

func TestLoad_ExecError(t *testing.T) {
	fe := &fakeExecer{err: errors.New("ssh: connection refused")}
	_, err := Load(context.Background(), fe, LoadParams{JumpHost: "jump", InventoryPath: "/x/inventory.yml"})
	if err == nil {
		t.Fatal("want error when execer fails, got nil")
	}
}
