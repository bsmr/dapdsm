package ssh

import (
	"context"
	"strings"
	"testing"
)

func TestConnectControlMasterArgs(t *testing.T) {
	f := &fakeRunner{out: Result{}}
	c := &Client{Runner: f}
	err := c.Connect(context.Background(), "vm-a", "/run/dunemgr/sock-vm-a")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if f.gotName != "ssh" {
		t.Errorf("invoked %q, want ssh", f.gotName)
	}
	joined := strings.Join(f.gotArgs, " ")
	for _, want := range []string{"-M", "-S", "/run/dunemgr/sock-vm-a", "-N", "vm-a", "-o", "BatchMode=yes"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q in %q", want, joined)
		}
	}
}

func TestOpenTunnelIssuesForward(t *testing.T) {
	f := &fakeRunner{out: Result{}}
	c := &Client{Runner: f}
	err := c.OpenTunnel(context.Background(), "vm-a", "/run/dunemgr/sock-vm-a", 9090, "127.0.0.1", 5432)
	if err != nil {
		t.Fatalf("OpenTunnel: %v", err)
	}
	joined := strings.Join(f.gotArgs, " ")
	for _, want := range []string{"-S", "/run/dunemgr/sock-vm-a", "-O", "forward", "-L", "127.0.0.1:9090:127.0.0.1:5432", "vm-a"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q in %q", want, joined)
		}
	}
}

func TestDisconnectIssuesExit(t *testing.T) {
	f := &fakeRunner{}
	c := &Client{Runner: f}
	err := c.Disconnect(context.Background(), "/run/dunemgr/sock-vm-a", "vm-a")
	if err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	joined := strings.Join(f.gotArgs, " ")
	for _, want := range []string{"-S", "/run/dunemgr/sock-vm-a", "-O", "exit", "vm-a"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q in %q", want, joined)
		}
	}
}

func TestAllocPortAssignsFreePort(t *testing.T) {
	p, err := AllocPort()
	if err != nil {
		t.Fatalf("AllocPort: %v", err)
	}
	if p <= 0 || p > 65535 {
		t.Errorf("AllocPort returned %d, want 1..65535", p)
	}
}

func TestConnectRejectsFlagSmugglingHost(t *testing.T) {
	c := &Client{Runner: &fakeRunner{}}
	err := c.Connect(context.Background(), "-evil", "/run/dunemgr/sock")
	if err == nil {
		t.Errorf("Connect with flag-prefixed host: want error, got nil")
	}
}

func TestOpenTunnelRejectsFlagSmugglingHost(t *testing.T) {
	c := &Client{Runner: &fakeRunner{}}
	err := c.OpenTunnel(context.Background(), "-evil", "/run/dunemgr/sock", 9090, "127.0.0.1", 5432)
	if err == nil {
		t.Errorf("OpenTunnel with flag-prefixed host: want error, got nil")
	}
}

func TestOpenTunnelRejectsFlagSmugglingTargetHost(t *testing.T) {
	c := &Client{Runner: &fakeRunner{}}
	err := c.OpenTunnel(context.Background(), "vm-a", "/run/dunemgr/sock", 9090, "-evil", 5432)
	if err == nil {
		t.Errorf("OpenTunnel with flag-prefixed targetHost: want error, got nil")
	}
}

func TestDisconnectRejectsFlagSmugglingHost(t *testing.T) {
	c := &Client{Runner: &fakeRunner{}}
	err := c.Disconnect(context.Background(), "/run/dunemgr/sock", "-evil")
	if err == nil {
		t.Errorf("Disconnect with flag-prefixed host: want error, got nil")
	}
}
