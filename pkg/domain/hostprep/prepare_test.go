package hostprep

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

func TestPreparePlan_DefaultsAndSteps(t *testing.T) {
	steps := PreparePlan(Opts{User: "dune", Kubeconfig: "/k/cfg"}, nil)
	if len(steps) != 3 {
		t.Fatalf("want 3 steps (user, keys, sudoers), got %d", len(steps))
	}
	joined := ""
	for _, s := range steps {
		joined += strings.Join(s.Argv, " ") + "\n"
	}
	// UID/GID default to 2000 when unset.
	if !strings.Contains(joined, "2000") {
		t.Errorf("expected default uid/gid 2000 in steps:\n%s", joined)
	}
	// idempotency guards present.
	if !strings.Contains(joined, "id -u dune") || !strings.Contains(joined, "getent group dune") {
		t.Errorf("expected idempotency guards:\n%s", joined)
	}
	// sudoers validated with visudo.
	if !strings.Contains(joined, "visudo -cf /etc/sudoers.d/dune") {
		t.Errorf("expected visudo validation:\n%s", joined)
	}
}

func TestPreparePlan_Migrate(t *testing.T) {
	steps := PreparePlan(Opts{User: "dune"}, []string{"/home/someuser/.kube/config"})
	if len(steps) != 4 {
		t.Fatalf("want 4 steps with one migrate, got %d", len(steps))
	}
	if !strings.Contains(strings.Join(steps[3].Argv, " "), "/home/someuser/.kube/config") {
		t.Errorf("migrate step missing source: %v", steps[3].Argv)
	}
}

func TestApply_DryRunRunsNothing(t *testing.T) {
	called := false
	r := &fakeRunner{jump: func(string, ...string) (ssh.Result, error) {
		called = true
		return ssh.Result{}, nil
	}}
	var out bytes.Buffer
	if err := Apply(context.Background(), r, PreparePlan(Opts{User: "dune"}, nil), true, &out); err != nil {
		t.Fatalf("Apply dry-run: %v", err)
	}
	if called {
		t.Error("dry-run must not execute any command")
	}
	if !strings.Contains(out.String(), "useradd") {
		t.Errorf("dry-run should print the commands, got: %s", out.String())
	}
}

func TestApply_RunsEachStep(t *testing.T) {
	var n int
	r := &fakeRunner{jump: func(string, ...string) (ssh.Result, error) {
		n++
		return ssh.Result{}, nil
	}}
	var out bytes.Buffer
	if err := Apply(context.Background(), r, PreparePlan(Opts{User: "dune"}, nil), false, &out); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if n != 3 {
		t.Errorf("want 3 executed steps, got %d", n)
	}
}
