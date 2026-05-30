package tunnel

import (
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

type recordingRunner struct {
	calls [][]string // sequence of args lists
	err   error
}

func (r *recordingRunner) Run(ctx context.Context, name string, args ...string) (ssh.Result, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	return ssh.Result{}, r.err
}

func (r *recordingRunner) RunWithStdin(_ context.Context, _ []byte, _ string, _ ...string) (ssh.Result, error) {
	return ssh.Result{}, nil
}

func TestConnectRunsControlMaster(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	rec := &recordingRunner{}
	m := &Manager{SSH: &ssh.Client{Runner: rec}}
	if err := m.Connect(context.Background(), "vm-a"); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if !m.IsConnected("vm-a") {
		t.Errorf("not IsConnected after Connect")
	}
	if len(rec.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(rec.calls))
	}
	joined := strings.Join(rec.calls[0], " ")
	for _, want := range []string{"ssh", "-M", "-S", "-N", "vm-a"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Connect ssh args missing %q: %q", want, joined)
		}
	}
}

func TestConnectIsIdempotent(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	rec := &recordingRunner{}
	m := &Manager{SSH: &ssh.Client{Runner: rec}}
	_ = m.Connect(context.Background(), "vm-a")
	_ = m.Connect(context.Background(), "vm-a")
	if len(rec.calls) != 1 {
		t.Errorf("second Connect ran ssh again: %d calls", len(rec.calls))
	}
}
