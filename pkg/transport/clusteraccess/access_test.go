package clusteraccess

import (
	"context"
	"strings"
	"testing"
)

func TestAccessNodes(t *testing.T) {
	d := &Descriptor{Nodes: []Node{
		{Name: "c-0", Role: RoleControlPlane},
		{Name: "w-0", Role: RoleWorker},
		{Name: "w-1", Role: RoleWorker},
	}}
	a := New(&fakeExecer{}, d)

	if got := a.Nodes(RoleWorker); len(got) != 2 {
		t.Errorf("workers = %d, want 2", len(got))
	}
	if got := a.Nodes(""); len(got) != 3 {
		t.Errorf("all = %d, want 3", len(got))
	}
}

func TestAccessOnNode(t *testing.T) {
	fe := &fakeExecer{stdout: "ok\n"}
	d := &Descriptor{
		JumpHost: "jump", Distro: "rke2",
		NodeUser: "installer", NodeKey: "/k/id_ed25519",
		Nodes: []Node{{Name: "w-0", Address: "10.0.0.8", Role: RoleWorker}},
	}
	a := New(fe, d)

	if _, err := a.OnNode(context.Background(), "w-0", "true"); err != nil {
		t.Fatalf("OnNode: %v", err)
	}
	want := []string{"jump", "ssh", "-i", "/k/id_ed25519", "-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no", "installer@10.0.0.8", "true"}
	got := fe.calls[0]
	if len(got) != len(want) {
		t.Fatalf("call = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("call = %v, want %v", got, want)
		}
	}
}

func TestAccessOnNode_UnknownNode(t *testing.T) {
	a := New(&fakeExecer{}, &Descriptor{Distro: "rke2"})
	if _, err := a.OnNode(context.Background(), "nope", "true"); err == nil {
		t.Fatal("expected error for unknown node")
	}
}

func TestAccessOnNode_NonRKE2(t *testing.T) {
	d := &Descriptor{Distro: "talos", Nodes: []Node{{Name: "w-0", Address: "10.0.0.8"}}}
	a := New(&fakeExecer{}, d)
	if _, err := a.OnNode(context.Background(), "w-0", "true"); err == nil {
		t.Fatal("expected error: talos has no ssh node access in S0")
	}
}

func TestAccessKubectl(t *testing.T) {
	fe := &fakeExecer{stdout: "NAME STATUS\n"}
	d := &Descriptor{JumpHost: "jump", Kubeconfig: "/k/cfg"}
	a := New(fe, d)

	res, err := a.Kubectl(context.Background(), "get", "nodes", "-o", "wide")
	if err != nil {
		t.Fatalf("Kubectl: %v", err)
	}
	if res.Stdout != "NAME STATUS\n" {
		t.Errorf("stdout = %q", res.Stdout)
	}
	if len(fe.calls) != 1 {
		t.Fatalf("calls = %d", len(fe.calls))
	}
	want := []string{"jump", "env", "KUBECONFIG=/k/cfg", "kubectl", "get", "nodes", "-o", "wide"}
	got := fe.calls[0]
	if len(got) != len(want) {
		t.Fatalf("call = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("call = %v, want %v", got, want)
		}
	}
}

func TestAccessOnJump(t *testing.T) {
	fe := &fakeExecer{stdout: "ok\n"}
	a := New(fe, &Descriptor{JumpHost: "jump"})
	if _, err := a.OnJump(context.Background(), "getent", "passwd", "dune"); err != nil {
		t.Fatalf("OnJump: %v", err)
	}
	want := []string{"jump", "getent", "passwd", "dune"}
	got := fe.calls[0]
	if len(got) != len(want) {
		t.Fatalf("call = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("call = %v, want %v", got, want)
		}
	}
}

func TestAccessOnJumpStdin(t *testing.T) {
	fe := &fakeExecer{}
	a := New(fe, &Descriptor{JumpHost: "jump"})
	if _, err := a.OnJumpStdin(context.Background(), []byte("Arrakis\n2\ntok\n"),
		"sudo", "-u", "dune", "bash", "/p/setup.sh"); err != nil {
		t.Fatalf("OnJumpStdin: %v", err)
	}
	if fe.stdinCall == nil {
		t.Fatal("RunWithStdin was not used")
	}
	c := fe.stdinCall
	if c.host != "jump" {
		t.Errorf("host = %q, want jump", c.host)
	}
	if string(c.stdin) != "Arrakis\n2\ntok\n" {
		t.Errorf("stdin = %q", c.stdin)
	}
	want := []string{"sudo", "-u", "dune", "bash", "/p/setup.sh"}
	got := append([]string{c.cmd}, c.args...)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("argv = %v, want %v", got, want)
	}
}

func TestKubectlStdin_PipesManifestWithKubeconfig(t *testing.T) {
	f := &fakeExecer{}
	d := &Descriptor{JumpHost: "jump", Kubeconfig: "/home/dune/kubeconfig", Distro: "rke2"}
	a := New(f, d)
	if _, err := a.KubectlStdin(context.Background(), []byte("apiVersion: v1\n"), "apply", "-f", "-"); err != nil {
		t.Fatalf("KubectlStdin: %v", err)
	}
	if f.stdinCall == nil {
		t.Fatal("RunWithStdin was not used")
	}
	c := f.stdinCall
	if c.host != "jump" {
		t.Errorf("host = %q, want jump", c.host)
	}
	if string(c.stdin) != "apiVersion: v1\n" {
		t.Errorf("stdin = %q", c.stdin)
	}
	// env KUBECONFIG=... kubectl apply -f -
	want := []string{"env", "KUBECONFIG=/home/dune/kubeconfig", "kubectl", "apply", "-f", "-"}
	got := append([]string{c.cmd}, c.args...)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("argv = %v, want %v", got, want)
	}
}
