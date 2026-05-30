package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestDBUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := dbCmd(context.Background(), nil, &stdout, &stderr); err == nil {
		t.Error("db no args: err=nil, want non-nil")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Errorf("missing usage hint: %q", stderr.String())
	}
}

func TestDBRejectsUnknownSubcommand(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	var stdout, stderr bytes.Buffer
	if err := dbCmd(context.Background(), []string{"vm-a", "frobnicate"}, &stdout, &stderr); err == nil {
		t.Error("db bogus sub: err=nil, want non-nil")
	}
}

func TestDBExecRequiresSQL(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	var stdout, stderr bytes.Buffer
	if err := dbCmd(context.Background(), []string{"vm-a", "exec"}, &stdout, &stderr); err == nil {
		t.Error("exec without sql: err=nil, want non-nil")
	}
}

func TestDBColumnsRequiresArgs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	var stdout, stderr bytes.Buffer
	if err := dbCmd(context.Background(), []string{"vm-a", "columns", "dune"}, &stdout, &stderr); err == nil {
		t.Error("columns with missing table: err=nil, want non-nil")
	}
}
