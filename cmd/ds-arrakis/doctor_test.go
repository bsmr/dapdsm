package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/domain/hostprep"
	"go.muehmer.eu/dapdsm/pkg/transport/clusteraccess"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// fakeRunner for the cmd package (package main has no access to hostprep's test fake).
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

// fixed returns a runner factory that always yields r (ignoring jump/kube).
func fixed(r hostprep.Runner) func(string, string) hostprep.Runner {
	return func(string, string) hostprep.Runner { return r }
}

func TestDoctorCmd_GateFailsStops(t *testing.T) {
	r := &fakeRunner{
		kube: func(args ...string) (ssh.Result, error) {
			return ssh.Result{Stdout: "no\n", ExitCode: 1}, &fakeExit{} // can-i no, get* error
		},
		jump: func(string, ...string) (ssh.Result, error) {
			t.Fatal("host checks must not run when the gate fails")
			return ssh.Result{}, nil
		},
	}
	var out, errOut bytes.Buffer
	err := doctorCmd(context.Background(), fixed(r), []string{"--jump", "j", "--kubeconfig", "/k"}, &out, &errOut)
	if err == nil {
		t.Fatal("expected an error when the cluster-access gate fails")
	}
	if !errors.Is(err, ErrGateFailed) {
		t.Errorf("expected ErrGateFailed, got: %v", err)
	}
	if !strings.Contains(out.String(), "cluster-access gate") {
		t.Errorf("expected gate probes printed on failure, got: %q", out.String())
	}
}

func TestDoctorCmd_GreenRunsHostChecks(t *testing.T) {
	r := &fakeRunner{
		kube: func(args ...string) (ssh.Result, error) {
			if len(args) >= 2 && args[0] == "auth" {
				return ssh.Result{Stdout: "yes\n"}, nil
			}
			return ssh.Result{Stdout: "ok\n"}, nil
		},
		jump: func(name string, args ...string) (ssh.Result, error) {
			if name == "id" {
				return ssh.Result{Stdout: "0\n"}, nil
			}
			if name == "getent" {
				return ssh.Result{Stdout: "dune:x:2000:2000::/home/dune:/bin/bash\n"}, nil
			}
			return ssh.Result{}, nil
		},
	}
	var out, errOut bytes.Buffer
	if err := doctorCmd(context.Background(), fixed(r), []string{"--jump", "j", "--kubeconfig", "/k"}, &out, &errOut); err != nil {
		t.Fatalf("doctorCmd: %v", err)
	}
	if !strings.Contains(out.String(), "user dune exists") {
		t.Errorf("expected host checks in output, got: %s", out.String())
	}
}

func TestDoctorCmd_MissingFlags(t *testing.T) {
	var out, errOut bytes.Buffer
	// newRunner must never be called when flags are invalid.
	nr := func(string, string) hostprep.Runner {
		t.Fatal("runner factory must not be called on a usage error")
		return nil
	}
	err := doctorCmd(context.Background(), nr, []string{"--jump", "j"}, &out, &errOut)
	if err == nil {
		t.Fatal("expected error when --kubeconfig is missing")
	}
}

type fakeExit struct{}

func (*fakeExit) Error() string { return "exit status 1" }

var _ hostprep.Runner = (*clusteraccess.Access)(nil)
