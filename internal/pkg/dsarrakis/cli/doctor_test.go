package cli

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
			// ControlPlaneTaint query 1: return empty CP node list.
			if len(args) >= 2 && args[0] == "get" && args[1] == "nodes" {
				return ssh.Result{Stdout: ""}, nil
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

func TestDoctorCmd_PrintsClusterSchedulingSection(t *testing.T) {
	// Multi-node cluster with an untainted CP node; doctor must print the
	// "cluster scheduling" section and be advisory (no error returned).
	kubeCalls := 0
	r := &fakeRunner{
		kube: func(args ...string) (ssh.Result, error) {
			kubeCalls++
			if len(args) >= 2 && args[0] == "auth" {
				return ssh.Result{Stdout: "yes\n"}, nil
			}
			if len(args) >= 2 && args[0] == "get" && args[1] == "nodes" {
				// Distinguish the two ControlPlaneTaint queries by label selector.
				joined := strings.Join(args, " ")
				if strings.Contains(joined, "!node-role") {
					// Worker query — return one worker.
					return ssh.Result{Stdout: "worker-00"}, nil
				}
				// CP query — one untainted CP node.
				return ssh.Result{Stdout: "cp-00=;"}, nil
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
	// Even with a failing CP taint check, doctorCmd must return nil (advisory).
	if err := doctorCmd(context.Background(), fixed(r), []string{"--jump", "j", "--kubeconfig", "/k"}, &out, &errOut); err != nil {
		t.Fatalf("doctorCmd must not return error for advisory checks, got: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "cluster scheduling") {
		t.Errorf("expected '== cluster scheduling ==' section in output, got:\n%s", got)
	}
	if !strings.Contains(got, "control-plane workload isolation") {
		t.Errorf("expected check name in output, got:\n%s", got)
	}
	// The check is failing ([!!]) because cp-00 is untainted.
	if !strings.Contains(got, "[!!]") {
		t.Errorf("expected [!!] marker for untainted CP node, got:\n%s", got)
	}
	if !strings.Contains(got, "cp-00") {
		t.Errorf("expected untainted node cp-00 named in output, got:\n%s", got)
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
