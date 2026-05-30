package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestBackupUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := backupCmd(context.Background(), nil, &stdout, &stderr); err == nil {
		t.Error("backup no args: err=nil, want non-nil")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Errorf("missing usage hint: %q", stderr.String())
	}
}

func TestBackupRejectsUnknownSubcommand(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	var stdout, stderr bytes.Buffer
	if err := backupCmd(context.Background(), []string{"vm-a", "bg", "frobnicate"}, &stdout, &stderr); err == nil {
		t.Error("backup bogus sub: err=nil, want non-nil")
	}
}

func TestBackupCreateRequiresName(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	var stdout, stderr bytes.Buffer
	if err := backupCmd(context.Background(), []string{"vm-a", "bg", "create"}, &stdout, &stderr); err == nil {
		t.Error("create without name: err=nil, want non-nil")
	}
}

func TestBackupRestoreRequiresKeyAndConfirm(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	var stdout, stderr bytes.Buffer
	if err := backupCmd(context.Background(), []string{"vm-a", "bg", "restore"}, &stdout, &stderr); err == nil {
		t.Error("restore without key: err=nil, want non-nil")
	}
	if err := backupCmd(context.Background(), []string{"vm-a", "bg", "restore", "key"}, &stdout, &stderr); err == nil {
		t.Error("restore without --confirm: err=nil, want non-nil")
	}
}
