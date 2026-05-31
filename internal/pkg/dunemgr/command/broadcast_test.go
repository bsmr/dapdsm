package command

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

func TestBroadcastUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := broadcastCmd(context.Background(), nil, nil, &stdout, &stderr); err == nil {
		t.Error("broadcast no args: err=nil, want non-nil")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Errorf("missing usage hint: %q", stderr.String())
	}
}

func TestBroadcastRejectsUnknownKind(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &core.Core{Store: st, SSH: ssh.NewClient()}

	var stdout, stderr bytes.Buffer
	if err := broadcastCmd(context.Background(), c, []string{"vm-a", "spam"}, &stdout, &stderr); err == nil {
		t.Error("broadcast bogus kind: err=nil, want non-nil")
	}
}

func TestBroadcastNoticeRequiresFlags(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &core.Core{Store: st, SSH: ssh.NewClient()}

	var stdout, stderr bytes.Buffer
	if err := broadcastCmd(context.Background(), c, []string{"vm-a", "notice"}, &stdout, &stderr); err == nil {
		t.Error("notice without --title/--body: err=nil, want non-nil")
	}
}
