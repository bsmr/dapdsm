package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesDefaultWhenMissing(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Bind != "127.0.0.1:8765" {
		t.Errorf("default Bind = %q, want %q", cfg.Bind, "127.0.0.1:8765")
	}
	// File created on disk
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err != nil {
		t.Errorf("config.yaml not written: %v", err)
	}
}

func TestLoadReadsExisting(t *testing.T) {
	dir := t.TempDir()
	body := "bind: 127.0.0.1:9999\ndefault_host: vm-x\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Bind != "127.0.0.1:9999" {
		t.Errorf("Bind = %q, want 127.0.0.1:9999", cfg.Bind)
	}
	if cfg.DefaultHost != "vm-x" {
		t.Errorf("DefaultHost = %q, want vm-x", cfg.DefaultHost)
	}
}
