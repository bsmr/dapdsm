package store

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenLockedReturnsClearError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.bolt")
	first, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	defer first.Close()

	_, err = Open(path) // second holder — must fail fast, not hang
	if err == nil {
		t.Fatal("second Open succeeded, want lock error")
	}
	if !strings.Contains(err.Error(), "database locked") {
		t.Errorf("error = %q, want it to mention 'database locked'", err)
	}
	// The hint must point at the current lock holder (the TUI, since the web UI
	// is disabled), not the removed "serve" verb.
	if !strings.Contains(err.Error(), "tui") {
		t.Errorf("error = %q, want it to hint at a running tui instance", err)
	}
	if strings.Contains(err.Error(), "serve") {
		t.Errorf("error = %q, must not reference the removed serve verb", err)
	}
}
