package cli

import (
	"path/filepath"
	"testing"
)

func TestConfigDirUsesXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdgcfg")
	got, err := configDir()
	if err != nil {
		t.Fatalf("configDir: %v", err)
	}
	if got != filepath.Join("/tmp/xdgcfg", "dunemgr") {
		t.Errorf("configDir = %q", got)
	}
}

func TestDataDirFallsBackToConfigDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdgcfg")
	t.Setenv("XDG_DATA_HOME", "")
	got, err := dataDir()
	if err != nil {
		t.Fatalf("dataDir: %v", err)
	}
	if got != filepath.Join("/tmp/xdgcfg", "dunemgr") {
		t.Errorf("dataDir = %q, want config-dir fallback", got)
	}
}

func TestDataDirUsesXDGData(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdgdata")
	got, err := dataDir()
	if err != nil {
		t.Fatalf("dataDir: %v", err)
	}
	if got != filepath.Join("/tmp/xdgdata", "dunemgr") {
		t.Errorf("dataDir = %q", got)
	}
}

func TestOpenStoreCreatesBolt(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	s, err := openStore()
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	defer s.Close()
	if _, err := s.ListHosts(); err != nil {
		t.Errorf("ListHosts on fresh store: %v", err)
	}
}
