package clusteraccess

import (
	"context"
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
