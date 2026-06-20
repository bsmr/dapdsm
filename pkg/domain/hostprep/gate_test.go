package hostprep

import (
	"context"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// fakeRunner returns a canned ssh.Result/error per kubectl arg signature.
type fakeRunner struct {
	kube func(args ...string) (ssh.Result, error)
	jump func(name string, args ...string) (ssh.Result, error)
}

func (f *fakeRunner) Kubectl(_ context.Context, args ...string) (ssh.Result, error) {
	return f.kube(args...)
}
func (f *fakeRunner) OnJump(_ context.Context, name string, args ...string) (ssh.Result, error) {
	return f.jump(name, args...)
}

func TestClusterGate_AllGreen(t *testing.T) {
	r := &fakeRunner{
		kube: func(args ...string) (ssh.Result, error) {
			if len(args) >= 2 && args[0] == "auth" && args[1] == "can-i" {
				// can-i create X -> yes, with a stderr warning that must be ignored.
				return ssh.Result{Stdout: "yes\n", Stderr: "Warning: resource not namespace scoped\n"}, nil
			}
			return ssh.Result{Stdout: "ok\n"}, nil // get ns / get nodes
		},
		jump: func(name string, args ...string) (ssh.Result, error) {
			panic("OnJump must not be called from a Phase-A gate test")
		},
	}
	g := ClusterGate(context.Background(), r)
	if !g.Pass {
		t.Fatalf("want Pass=true, got %+v", g)
	}
	if len(g.Probes) != 5 {
		t.Errorf("want 5 probes, got %d", len(g.Probes))
	}
}

func TestClusterGate_CanINo_FailsOnStdoutNotExit(t *testing.T) {
	// `auth can-i` answering "no" exits non-zero; the gate must decide on stdout.
	r := &fakeRunner{
		kube: func(args ...string) (ssh.Result, error) {
			if len(args) >= 2 && args[0] == "auth" && args[1] == "can-i" {
				return ssh.Result{Stdout: "no\n", ExitCode: 1}, &exitErr{}
			}
			return ssh.Result{Stdout: "ok\n"}, nil
		},
		jump: func(name string, args ...string) (ssh.Result, error) {
			panic("OnJump must not be called from a Phase-A gate test")
		},
	}
	g := ClusterGate(context.Background(), r)
	if g.Pass {
		t.Fatal("want Pass=false when can-i returns no")
	}
}

func TestClusterGate_GetNodesForbidden(t *testing.T) {
	// namespace-only SA: get nodes errors -> gate fails.
	r := &fakeRunner{
		kube: func(args ...string) (ssh.Result, error) {
			if len(args) >= 1 && args[0] == "get" && len(args) >= 2 && args[1] == "nodes" {
				return ssh.Result{Stderr: "forbidden", ExitCode: 1}, &exitErr{}
			}
			if len(args) >= 2 && args[0] == "auth" {
				return ssh.Result{Stdout: "yes\n"}, nil
			}
			return ssh.Result{Stdout: "ok\n"}, nil
		},
		jump: func(name string, args ...string) (ssh.Result, error) {
			panic("OnJump must not be called from a Phase-A gate test")
		},
	}
	g := ClusterGate(context.Background(), r)
	if g.Pass {
		t.Fatal("want Pass=false when get nodes is forbidden")
	}
	if g.Probes[1].Detail != "forbidden" {
		t.Errorf("Detail = %q, want %q", g.Probes[1].Detail, "forbidden")
	}
}

type exitErr struct{}

func (*exitErr) Error() string { return "exit status 1" }
