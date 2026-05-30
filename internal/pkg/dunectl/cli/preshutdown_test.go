package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type preShutdownRunner struct {
	statusReplies []string // consumed in order; last reply repeats on further calls
	podsReplies   []string // same
	getCallCount  int
}

func (r *preShutdownRunner) Get(_ context.Context, args ...string) ([]byte, error) {
	r.getCallCount++
	joined := strings.Join(args, " ")
	// "get battlegroup … -o jsonpath" → status reply
	if strings.Contains(joined, "battlegroup") && strings.Contains(joined, "phase") {
		if len(r.statusReplies) == 0 {
			return []byte("Healthy"), nil
		}
		out := r.statusReplies[0]
		if len(r.statusReplies) > 1 {
			r.statusReplies = r.statusReplies[1:]
		}
		return []byte(out), nil
	}
	// "get pods … -l role=igw-server" → pod list (empty = drained)
	if strings.Contains(joined, "pods") {
		if len(r.podsReplies) == 0 {
			return []byte(""), nil
		}
		out := r.podsReplies[0]
		if len(r.podsReplies) > 1 {
			r.podsReplies = r.podsReplies[1:]
		}
		return []byte(out), nil
	}
	// namespace lookup
	if len(args) >= 1 && args[0] == "ns" {
		return []byte("funcom-seabass-sh-deadbeef\n"), nil
	}
	return nil, errors.New("unexpected Get " + joined)
}

func (r *preShutdownRunner) Patch(context.Context, string, string, string, string, string) error {
	return nil
}
func (r *preShutdownRunner) DeletePods(context.Context, string, ...string) error { return nil }
func (r *preShutdownRunner) Exec(context.Context, string, string, ...string) ([]byte, error) {
	return nil, nil
}
func (r *preShutdownRunner) ExecPiped(context.Context, string, string, []byte, ...string) ([]byte, error) {
	return nil, nil
}

type recordedVendor struct{ actions []string }

func (r *recordedVendor) run(_ context.Context, _, action string, _, _ io.Writer) error {
	r.actions = append(r.actions, action)
	return nil
}

func TestPreShutdown_CallsStopThenWaitsForDrain(t *testing.T) {
	t.Parallel()
	v := &recordedVendor{}
	r := &preShutdownRunner{
		statusReplies: []string{"Healthy", "Stopping", "Stopped"},
		podsReplies:   []string{"pod-a pod-b", ""}, // first poll: 2 pods; second: drained
	}
	deps := preShutdownDeps{runner: r, runVendor: v.run, pollEvery: time.Millisecond}
	var stdout, stderr bytes.Buffer
	if err := runPreShutdown(context.Background(), nil, &stdout, &stderr, deps); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(v.actions) != 1 || v.actions[0] != "stop" {
		t.Errorf("vendor actions = %v, want [stop]", v.actions)
	}
	if !strings.Contains(stdout.String(), "pre-shutdown") {
		t.Errorf("stdout missing summary: %q", stdout.String())
	}
}

func TestPreShutdown_TimeoutSurfacesAsWarningNotError(t *testing.T) {
	t.Parallel()
	// status never reaches Stopped → polling timeout. Must NOT return
	// error; systemd would interpret that as ExecStop failure and may
	// kill k3s before drain completes.
	v := &recordedVendor{}
	r := &preShutdownRunner{
		statusReplies: []string{"Stopping"},    // stays "Stopping" forever
		podsReplies:   []string{"pod-a pod-b"}, // stays unhealthy
	}
	deps := preShutdownDeps{
		runner:    r,
		runVendor: v.run,
		pollEvery: time.Millisecond,
		timeout:   5 * time.Millisecond,
	}
	var stdout, stderr bytes.Buffer
	err := runPreShutdown(context.Background(), nil, &stdout, &stderr, deps)
	if err != nil {
		t.Fatalf("err = %v, want nil (timeout should warn, not fail)", err)
	}
	if !strings.Contains(stderr.String(), "timeout") && !strings.Contains(stdout.String(), "timeout") {
		t.Errorf("expected a timeout warning somewhere; stdout=%q stderr=%q",
			stdout.String(), stderr.String())
	}
}

func TestPreShutdown_PropagatesVendorStopError(t *testing.T) {
	t.Parallel()
	v := failingVendor{err: errors.New("battlegroup binary missing")}
	r := &preShutdownRunner{}
	deps := preShutdownDeps{runner: r, runVendor: v.run, pollEvery: time.Millisecond}
	var stdout, stderr bytes.Buffer
	err := runPreShutdown(context.Background(), nil, &stdout, &stderr, deps)
	if err == nil {
		t.Errorf("err = nil, want propagated vendor error")
	}
}

type failingVendor struct{ err error }

func (f failingVendor) run(_ context.Context, _, _ string, _, _ io.Writer) error { return f.err }
