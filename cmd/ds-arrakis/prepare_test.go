package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

func TestPrepareCmd_GatePrecondition(t *testing.T) {
	applied := false
	r := &fakeRunner{
		kube: func(args ...string) (ssh.Result, error) {
			return ssh.Result{Stdout: "no\n", ExitCode: 1}, &fakeExit{} // gate fails
		},
		jump: func(string, ...string) (ssh.Result, error) {
			applied = true
			return ssh.Result{}, nil
		},
	}
	var out, errOut bytes.Buffer
	err := prepareCmd(context.Background(), fixed(r), []string{"--jump", "j", "--kubeconfig", "/k"}, &out, &errOut)
	if err == nil {
		t.Fatal("expected error: gate precondition must block prepare")
	}
	if applied {
		t.Error("no remediation may run when the gate fails")
	}
}

func TestPrepareCmd_DryRun(t *testing.T) {
	ran := false
	r := &fakeRunner{
		kube: func(args ...string) (ssh.Result, error) {
			if len(args) >= 2 && args[0] == "auth" {
				return ssh.Result{Stdout: "yes\n"}, nil
			}
			return ssh.Result{Stdout: "ok\n"}, nil
		},
		jump: func(string, ...string) (ssh.Result, error) {
			ran = true
			return ssh.Result{}, nil
		},
	}
	var out, errOut bytes.Buffer
	err := prepareCmd(context.Background(), fixed(r),
		[]string{"--jump", "j", "--kubeconfig", "/k", "--dry-run"}, &out, &errOut)
	if err != nil {
		t.Fatalf("prepareCmd dry-run: %v", err)
	}
	if ran {
		t.Error("dry-run must not execute remediation")
	}
	if !strings.Contains(out.String(), "useradd") {
		t.Errorf("dry-run should print planned commands, got: %s", out.String())
	}
}
