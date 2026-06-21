package kubeaccess

import (
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/clusteraccess"
	"go.muehmer.eu/dapdsm/pkg/transport/kube"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// recordingExecer captures the last argv and returns a canned stdout.
type recordingExecer struct {
	gotCmd  string
	gotArgs []string
	gotIn   []byte
	stdout  string
}

func (r *recordingExecer) Run(_ context.Context, _ string, cmd string, args ...string) (ssh.Result, error) {
	r.gotCmd, r.gotArgs = cmd, args
	return ssh.Result{Stdout: r.stdout}, nil
}

func (r *recordingExecer) RunWithStdin(_ context.Context, _ string, in []byte, cmd string, args ...string) (ssh.Result, error) {
	r.gotCmd, r.gotArgs, r.gotIn = cmd, args, in
	return ssh.Result{Stdout: r.stdout}, nil
}

func newAccess(ex clusteraccess.Execer) *clusteraccess.Access {
	return clusteraccess.New(ex, &clusteraccess.Descriptor{JumpHost: "jh", Kubeconfig: "/home/dune/kubeconfig", Distro: "rke2"})
}

func TestRunner_SatisfiesKubeRunner(t *testing.T) {
	var _ kube.Runner = New(newAccess(&recordingExecer{}))
}

func TestRunner_Get_PassesArgsAndReturnsStdout(t *testing.T) {
	ex := &recordingExecer{stdout: "node-list"}
	// Convention A: caller passes resource only; Get prepends "get".
	out, err := New(newAccess(ex)).Get(context.Background(), "nodes")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(out) != "node-list" {
		t.Fatalf("stdout = %q, want node-list", out)
	}
	// Access.Kubectl prepends env + KUBECONFIG + kubectl; assert our verb survived.
	joined := strings.Join(ex.gotArgs, " ")
	if !strings.Contains(joined, "kubectl get nodes") {
		t.Fatalf("argv %q missing 'kubectl get nodes'", joined)
	}
}

// TestRunner_Get_PrependsGet locks the CmdRunner-mirror convention: callers
// never include "get" themselves — Runner.Get inserts it automatically.
func TestRunner_Get_PrependsGet(t *testing.T) {
	ex := &recordingExecer{stdout: "ns-list"}
	New(newAccess(ex)).Get(context.Background(), "ns", "-o", "name") //nolint:errcheck
	joined := strings.Join(ex.gotArgs, " ")
	if !strings.Contains(joined, "kubectl get ns -o name") {
		t.Fatalf("argv %q: want 'kubectl get ns -o name'", joined)
	}
}

func TestRunner_ExecPiped_SendsStdin(t *testing.T) {
	ex := &recordingExecer{}
	_, err := New(newAccess(ex)).ExecPiped(context.Background(), "ns", "pod", []byte("SELECT 1"), "psql")
	if err != nil {
		t.Fatalf("ExecPiped: %v", err)
	}
	if string(ex.gotIn) != "SELECT 1" {
		t.Fatalf("stdin = %q, want 'SELECT 1'", ex.gotIn)
	}
	joined := strings.Join(ex.gotArgs, " ")
	if !strings.Contains(joined, "kubectl exec -i pod -n ns -- psql") {
		t.Fatalf("argv %q missing exec form", joined)
	}
}
