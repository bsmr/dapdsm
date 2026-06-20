package hostprep

import (
	"context"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

func TestHostChecks(t *testing.T) {
	// All host commands succeed -> all checks OK.
	r := &fakeRunner{jump: func(name string, args ...string) (ssh.Result, error) {
		if name == "id" {
			return ssh.Result{Stdout: "0\n"}, nil // root
		}
		if name == "getent" {
			return ssh.Result{Stdout: "dune:x:2000:2000::/home/dune:/bin/bash\n"}, nil
		}
		return ssh.Result{}, nil // sudo test ... succeed
	}}
	checks := HostChecks(context.Background(), r, Opts{User: "dune", Kubeconfig: "/k/cfg"})
	if len(checks) != 5 {
		t.Fatalf("want 5 checks, got %d", len(checks))
	}
	for _, c := range checks {
		if !c.OK {
			t.Errorf("check %q not OK: %s", c.Name, c.Detail)
		}
	}
}

func TestHostChecks_DuneAbsent(t *testing.T) {
	r := &fakeRunner{jump: func(name string, args ...string) (ssh.Result, error) {
		if name == "id" {
			return ssh.Result{Stdout: "1000\n"}, nil // not root
		}
		if name == "sudo" && len(args) >= 1 && args[0] == "-n" {
			return ssh.Result{}, nil // passwordless sudoer ok
		}
		if name == "getent" {
			return ssh.Result{ExitCode: 2}, &exitErr{} // dune absent
		}
		return ssh.Result{ExitCode: 1}, &exitErr{} // dune-dependent checks fail
	}}
	checks := HostChecks(context.Background(), r, Opts{User: "dune", Kubeconfig: "/k/cfg"})
	byName := map[string]Check{}
	for _, c := range checks {
		byName[c.Name] = c
	}
	if !byName["privilege"].OK {
		t.Error("privilege should be OK (passwordless sudoer)")
	}
	if byName["user dune exists"].OK {
		t.Error("user dune should be reported absent")
	}
}
