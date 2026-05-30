package tunnel

import (
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

func TestDisconnectClosesMaster(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	rec := &recordingRunner{}
	m := &Manager{SSH: &ssh.Client{Runner: rec}}
	_ = m.Connect(context.Background(), "vm-a")
	if err := m.Disconnect(context.Background(), "vm-a"); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	if m.IsConnected("vm-a") {
		t.Errorf("still IsConnected after Disconnect")
	}
	// last ssh call must be -O exit
	last := rec.calls[len(rec.calls)-1]
	if !strings.Contains(strings.Join(last, " "), "-O exit") {
		t.Errorf("last ssh call missing -O exit: %v", last)
	}
}

func TestDisconnectUnknownHostIsNoOp(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	rec := &recordingRunner{}
	m := &Manager{SSH: &ssh.Client{Runner: rec}}
	if err := m.Disconnect(context.Background(), "ghost"); err != nil {
		t.Errorf("Disconnect of unknown host: %v", err)
	}
	if len(rec.calls) != 0 {
		t.Errorf("Disconnect of unknown ran ssh: %v", rec.calls)
	}
}
