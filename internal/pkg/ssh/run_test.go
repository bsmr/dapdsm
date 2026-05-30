package ssh

import (
	"context"
	"errors"
	"testing"
)

type fakeRunner struct {
	gotName string
	gotArgs []string
	out     Result
	err     error
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	f.gotName = name
	f.gotArgs = args
	return f.out, f.err
}

func (f *fakeRunner) RunWithStdin(_ context.Context, _ []byte, _ string, _ ...string) (Result, error) {
	return Result{}, nil
}

func TestRunPassesBatchModeAndHost(t *testing.T) {
	f := &fakeRunner{out: Result{Stdout: "ok\n"}}
	c := &Client{Runner: f}
	res, err := c.Run(context.Background(), "vm-a", "echo", "ok")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stdout != "ok\n" {
		t.Errorf("stdout = %q, want %q", res.Stdout, "ok\n")
	}
	if f.gotName != "ssh" {
		t.Errorf("invoked %q, want ssh", f.gotName)
	}
	wantPrefix := []string{"-o", "BatchMode=yes", "--", "vm-a", "echo", "ok"}
	if len(f.gotArgs) != len(wantPrefix) {
		t.Fatalf("args = %v, want %v", f.gotArgs, wantPrefix)
	}
	for i := range wantPrefix {
		if f.gotArgs[i] != wantPrefix[i] {
			t.Errorf("arg[%d] = %q, want %q", i, f.gotArgs[i], wantPrefix[i])
		}
	}
}

func TestRunPropagatesError(t *testing.T) {
	want := errors.New("boom")
	f := &fakeRunner{err: want}
	c := &Client{Runner: f}
	_, err := c.Run(context.Background(), "vm-a", "false")
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}

func TestRunRejectsFlagSmugglingHost(t *testing.T) {
	c := &Client{Runner: &fakeRunner{}}
	_, err := c.Run(context.Background(), "-oProxyCommand=evil", "echo", "x")
	if err == nil {
		t.Errorf("Run with flag-prefixed host: want error, got nil")
	}
}
