package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestHostListEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	var stdout, stderr bytes.Buffer
	if err := hostCmd(context.Background(), []string{"list"}, &stdout, &stderr); err != nil {
		t.Fatalf("host list (empty): %v", err)
	}
	if !strings.Contains(stdout.String(), "NAME") {
		t.Errorf("host list output missing header: %q", stdout.String())
	}
}

func TestHostUnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := hostCmd(context.Background(), []string{"nope"}, &stdout, &stderr); err == nil {
		t.Error("host nope: want error, got nil")
	}
}

func TestHostProbeUnknownHost(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	var stdout, stderr bytes.Buffer
	if err := hostCmd(context.Background(), []string{"probe", "ghost"}, &stdout, &stderr); err == nil {
		t.Error("probe of unknown host: want error, got nil")
	}
}

func TestHostNoArgsIsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := hostCmd(context.Background(), nil, &stdout, &stderr); err == nil {
		t.Error("host with no args: want error, got nil")
	}
}
