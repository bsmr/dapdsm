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
	// First arg must be "ssh", then BatchMode, then -- vm-a kubectl exec ...
	if rr.gotArgs[0] != "ssh" {
		t.Errorf("argv[0]=%q, want ssh", rr.gotArgs[0])
	}
	got := joinArgs(rr.gotArgs)
	if !contains(got, "vm-a kubectl exec -i podname -- cat") {
		t.Errorf("argv tail mismatch: %q", got)
	}
}

func TestRunWithStdinRejectsFlagHost(t *testing.T) {
	c := &Client{Runner: &recordingStdinRunner{}}
	_, err := c.RunWithStdin(context.Background(), "-evil", nil, "kubectl")
	if err == nil {
		t.Error("RunWithStdin with flag-host err=nil, want non-nil")
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
