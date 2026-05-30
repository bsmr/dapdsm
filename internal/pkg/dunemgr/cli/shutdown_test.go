package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestShutdownUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := shutdownCmd(context.Background(), nil, &stdout, &stderr); err == nil {
		t.Error("shutdown no args: err=nil, want non-nil")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Errorf("missing usage: %q", stderr.String())
	}
}

func TestShutdownUnknownSub(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	var stdout, stderr bytes.Buffer
	if err := shutdownCmd(context.Background(), []string{"vm-a", "frobnicate"}, &stdout, &stderr); err == nil {
		t.Error("shutdown bogus sub: err=nil, want non-nil")
	}
}
