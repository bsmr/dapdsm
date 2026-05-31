package core

import (
	"path/filepath"
	"testing"
)

func tempGetenv(dir string) func(string) string {
	return func(k string) string {
		switch k {
		case "XDG_CONFIG_HOME", "XDG_DATA_HOME":
			return dir
		default:
			return ""
		}
	}
}

func TestOpenWiresAllDeps(t *testing.T) {
	dir := t.TempDir()
	c, err := Open(tempGetenv(dir))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer c.Close()

	if c.Store == nil || c.SSH == nil || c.Hub == nil || c.Poller == nil || c.Schedule == nil {
		t.Fatalf("Open left a nil dep: %+v", c)
	}
	wantDir := filepath.Join(dir, "dunemgr")
	if c.Dir != wantDir {
		t.Errorf("Dir = %q, want %q", c.Dir, wantDir)
	}
	if c.DataDir != wantDir {
		t.Errorf("DataDir = %q, want %q", c.DataDir, wantDir)
	}
	if c.BackupDir != filepath.Join(wantDir, "backups") {
		t.Errorf("BackupDir = %q, want %q", c.BackupDir, filepath.Join(wantDir, "backups"))
	}
	if c.Poller.Hub != c.Hub {
		t.Error("Poller.Hub is not the Core Hub")
	}
}

func TestCloseReleasesStoreLock(t *testing.T) {
	dir := t.TempDir()
	c, err := Open(tempGetenv(dir))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	c2, err := Open(tempGetenv(dir))
	if err != nil {
		t.Fatalf("second Open after Close: %v", err)
	}
	c2.Close()
}

func TestConfigDirFallsBackToHomeWhenXDGUnset(t *testing.T) {
	getenv := func(string) string { return "" }
	got, err := ConfigDir(getenv)
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}
	if filepath.Base(got) != "dunemgr" {
		t.Errorf("ConfigDir = %q, want it to end in /dunemgr", got)
	}
}
