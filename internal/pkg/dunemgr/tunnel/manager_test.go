package tunnel

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeDirHonorsXDG(t *testing.T) {
	want := filepath.Join(t.TempDir(), "explicit-runtime")
	t.Setenv("XDG_RUNTIME_DIR", want)
	got := RuntimeDir()
	if !strings.HasPrefix(got, want) {
		t.Errorf("RuntimeDir = %q, want prefix %q", got, want)
	}
	if !strings.HasSuffix(got, "/dunemgr") {
		t.Errorf("RuntimeDir = %q, want /dunemgr suffix", got)
	}
}

func TestRuntimeDirFallback(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	got := RuntimeDir()
	if got == "" {
		t.Errorf("RuntimeDir empty fallback path")
	}
}

func TestSockPathPerHost(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	if a, b := SockPath("vm-a"), SockPath("vm-b"); a == b {
		t.Errorf("SockPath collision: %q == %q", a, b)
	}
}

func TestManagerZeroValue(t *testing.T) {
	var m Manager
	if m.IsConnected("anyone") {
		t.Errorf("zero-value Manager reports connected")
	}
}
