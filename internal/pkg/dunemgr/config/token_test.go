package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureTokenCreatesNew(t *testing.T) {
	dir := t.TempDir()
	tok, err := EnsureToken(dir)
	if err != nil {
		t.Fatalf("EnsureToken: %v", err)
	}
	if len(tok) < 32 {
		t.Errorf("token too short: %d chars", len(tok))
	}
	// Reading the file directly should give the same token.
	data, _ := os.ReadFile(filepath.Join(dir, "token"))
	if string(data) != tok {
		t.Errorf("token file mismatch: %q vs %q", string(data), tok)
	}
}

func TestEnsureTokenIdempotent(t *testing.T) {
	dir := t.TempDir()
	a, _ := EnsureToken(dir)
	b, _ := EnsureToken(dir)
	if a != b {
		t.Errorf("EnsureToken not idempotent: %q != %q", a, b)
	}
}

func TestRegenTokenReplaces(t *testing.T) {
	dir := t.TempDir()
	a, _ := EnsureToken(dir)
	b, err := RegenToken(dir)
	if err != nil {
		t.Fatalf("RegenToken: %v", err)
	}
	if a == b {
		t.Errorf("RegenToken did not change token")
	}
}

func TestTokenFilePermissions(t *testing.T) {
	dir := t.TempDir()
	_, _ = EnsureToken(dir)
	info, _ := os.Stat(filepath.Join(dir, "token"))
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perms = %o, want 0600", info.Mode().Perm())
	}
}
