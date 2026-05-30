package tunnel

import (
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

func TestOpenSlotIssuesForward(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	rec := &recordingRunner{}
	m := &Manager{SSH: &ssh.Client{Runner: rec}}
	_ = m.Connect(context.Background(), "vm-a")
	port, err := m.OpenSlot(context.Background(), "vm-a", "127.0.0.1", 6443)
	if err != nil {
		t.Fatalf("OpenSlot: %v", err)
	}
	if port <= 0 || port > 65535 {
		t.Errorf("port = %d, want 1..65535", port)
	}
	// last call must be a forward
	last := strings.Join(rec.calls[len(rec.calls)-1], " ")
	if !strings.Contains(last, "-O forward") {
		t.Errorf("last ssh missing -O forward: %q", last)
	}
}

func TestOpenSlotReusesExisting(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	rec := &recordingRunner{}
	m := &Manager{SSH: &ssh.Client{Runner: rec}}
	_ = m.Connect(context.Background(), "vm-a")
	p1, _ := m.OpenSlot(context.Background(), "vm-a", "127.0.0.1", 6443)
	beforeCalls := len(rec.calls)
	p2, _ := m.OpenSlot(context.Background(), "vm-a", "127.0.0.1", 6443)
	if p1 != p2 {
		t.Errorf("OpenSlot same target: ports %d != %d", p1, p2)
	}
	if len(rec.calls) != beforeCalls {
		t.Errorf("OpenSlot same target ran ssh again")
	}
}

func TestOpenSlotRequiresConnect(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	rec := &recordingRunner{}
	m := &Manager{SSH: &ssh.Client{Runner: rec}}
	_, err := m.OpenSlot(context.Background(), "ghost", "127.0.0.1", 6443)
	if err == nil {
		t.Errorf("OpenSlot on unconnected host: want error, got nil")
	}
}
