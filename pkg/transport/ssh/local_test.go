package ssh

import (
	"context"
	"testing"
)

// localFakeRunner records the bare command it was asked to run.
// Distinct from fakeRunner in run_test.go (which simulates ssh.Client's inner runner).
type localFakeRunner struct {
	gotName string
	gotArgs []string
	gotIn   []byte
}

func (f *localFakeRunner) Run(_ context.Context, name string, args ...string) (Result, error) {
	f.gotName, f.gotArgs = name, args
	return Result{Stdout: "ok"}, nil
}
func (f *localFakeRunner) RunWithStdin(_ context.Context, stdin []byte, name string, args ...string) (Result, error) {
	f.gotName, f.gotArgs, f.gotIn = name, args, stdin
	return Result{Stdout: "ok"}, nil
}

func TestLocalExecerIgnoresHostAndRunsDirectly(t *testing.T) {
	fr := &localFakeRunner{}
	le := LocalExecer{Runner: fr}

	if _, err := le.Run(context.Background(), "some-host", "kubectl", "get", "pods"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fr.gotName != "kubectl" || len(fr.gotArgs) != 2 || fr.gotArgs[0] != "get" {
		t.Fatalf("host not stripped / wrong command: %q %v", fr.gotName, fr.gotArgs)
	}

	if _, err := le.RunWithStdin(context.Background(), "host", []byte("IN"), "sh", "-c", "x"); err != nil {
		t.Fatalf("RunWithStdin: %v", err)
	}
	if string(fr.gotIn) != "IN" || fr.gotName != "sh" {
		t.Fatalf("stdin/command not forwarded: %q %q", fr.gotIn, fr.gotName)
	}
}
