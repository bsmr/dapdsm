package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestBroadcastUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := broadcastCmd(context.Background(), nil, &stdout, &stderr); err == nil {
		t.Error("broadcast no args: err=nil, want non-nil")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Errorf("missing usage hint: %q", stderr.String())
	}
}

func TestBroadcastRejectsUnknownKind(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	var stdout, stderr bytes.Buffer
	if err := broadcastCmd(context.Background(), []string{"vm-a", "spam"}, &stdout, &stderr); err == nil {
		t.Error("broadcast bogus kind: err=nil, want non-nil")
	}
}

func TestBroadcastNoticeRequiresFlags(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	var stdout, stderr bytes.Buffer
	if err := broadcastCmd(context.Background(), []string{"vm-a", "notice"}, &stdout, &stderr); err == nil {
		t.Error("notice without --title/--body: err=nil, want non-nil")
	}
}
