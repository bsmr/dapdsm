package ssh

import (
	"bytes"
	"context"
	"testing"
)

type recordingStdinRunner struct {
	gotStdin []byte
	gotArgs  []string
}

func (r *recordingStdinRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	r.gotArgs = append(r.gotArgs, name)
	r.gotArgs = append(r.gotArgs, args...)
	return Result{}, nil
}

func (r *recordingStdinRunner) RunWithStdin(ctx context.Context, stdin []byte, name string, args ...string) (Result, error) {
	r.gotStdin = append([]byte(nil), stdin...)
	r.gotArgs = append([]string{name}, args...)
	return Result{Stdout: "ok\n"}, nil
}

func TestRunWithStdinPipesPayload(t *testing.T) {
	rr := &recordingStdinRunner{}
	c := &Client{Runner: rr}
	payload := []byte("hello erlang.\n")

	_, err := c.RunWithStdin(context.Background(), "vm-a", payload, "kubectl", "exec", "-i", "podname", "--", "cat")
	if err != nil {
		t.Fatalf("RunWithStdin: %v", err)
	}
	if !bytes.Equal(rr.gotStdin, payload) {
		t.Errorf("stdin=%q, want %q", rr.gotStdin, payload)
	}
	// After the fix argv is: ["ssh", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=accept-new", "--", "vm-a", <single remote token>]
	// — 8 elements total, with the remote command as the last one.
	if rr.gotArgs[0] != "ssh" {
		t.Errorf("argv[0]=%q, want ssh", rr.gotArgs[0])
	}
	if len(rr.gotArgs) != 8 {
		t.Fatalf("argv len=%d, want 8; argv=%v", len(rr.gotArgs), rr.gotArgs)
	}
	if rr.gotArgs[6] != "vm-a" {
		t.Errorf("argv[6]=%q, want vm-a", rr.gotArgs[6])
	}
	remoteArg := rr.gotArgs[7]
	// Every component of the original command must appear in the single quoted remote token.
	for _, want := range []string{"kubectl", "exec", "-i", "podname", "--", "cat"} {
		if !contains(remoteArg, want) {
			t.Errorf("remote arg %q missing component %q", remoteArg, want)
		}
	}
}

func TestRunWithStdinRejectsFlagHost(t *testing.T) {
	c := &Client{Runner: &recordingStdinRunner{}}
	_, err := c.RunWithStdin(context.Background(), "-evil", nil, "kubectl")
	if err == nil {
		t.Error("RunWithStdin with flag-host err=nil, want non-nil")
	}
}

// TestRunWithStdinRemoteArgIsSingleToken verifies that a multi-word arg
// ends up as ONE element in the argv passed to the runner.
func TestRunWithStdinRemoteArgIsSingleToken(t *testing.T) {
	rr := &recordingStdinRunner{}
	c := &Client{Runner: rr}
	_, _ = c.RunWithStdin(context.Background(), "vm-a", nil, "kubectl", "exec", "-i", "pod", "--", "sh", "-lc", "echo foo; echo bar")

	// argv: ["ssh", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=accept-new", "--", "vm-a", <remote>]
	if len(rr.gotArgs) != 8 {
		t.Fatalf("argv len=%d, want 8; argv=%v", len(rr.gotArgs), rr.gotArgs)
	}
	remoteArg := rr.gotArgs[7]
	// The multi-statement shell script must survive as one intact token.
	if !contains(remoteArg, "echo foo; echo bar") {
		t.Errorf("shell script not intact in remote arg: %q", remoteArg)
	}
}

// joinArgs / contains: small string helpers for argv-assertion.
func joinArgs(a []string) string {
	out := ""
	for i, s := range a {
		if i > 0 {
			out += " "
		}
		out += s
	}
	return out
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}
