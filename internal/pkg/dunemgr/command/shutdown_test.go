package command

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/pkg/domain/broadcast"
	"go.muehmer.eu/dapdsm/pkg/domain/lifecycle"
	"go.muehmer.eu/dapdsm/pkg/domain/schedule"
	"go.muehmer.eu/dapdsm/pkg/domain/store"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

func TestShutdownUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := shutdownCmd(context.Background(), nil, nil, &stdout, &stderr); err == nil {
		t.Error("shutdown no args: err=nil, want non-nil")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Errorf("missing usage: %q", stderr.String())
	}
}

func TestShutdownUnknownSub(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &core.Core{
		Store: st, SSH: ssh.NewClient(),
		Schedule: schedule.NewManager(
			&broadcast.Runner{Exec: nil, Store: st},
			&lifecycle.Runner{SSH: nil, Store: st},
			st, nil),
	}
	var stdout, stderr bytes.Buffer
	if err := shutdownCmd(context.Background(), c, []string{"vm-a", "frobnicate"}, &stdout, &stderr); err == nil {
		t.Error("shutdown bogus sub: err=nil, want non-nil")
	}
}

func TestShutdownStatusNoRecord(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &core.Core{
		Store: st, SSH: ssh.NewClient(),
		Schedule: schedule.NewManager(
			&broadcast.Runner{Exec: nil, Store: st},
			&lifecycle.Runner{SSH: nil, Store: st},
			st, nil),
	}
	var stdout, stderr bytes.Buffer
	if err := shutdownCmd(context.Background(), c, []string{"vm-a", "status"}, &stdout, &stderr); err != nil {
		t.Errorf("status with no record: unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "no pending shutdown") {
		t.Errorf("expected 'no pending shutdown', got: %q", stdout.String())
	}
}

func TestShutdownStatusWithRecord(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	// Seed a scheduled shutdown record directly in the store.
	if err := st.PutSchedule(store.ScheduledShutdown{
		Host: "vm-a", Kind: "Restart", Action: "stop", AtUnix: 9999999999,
	}); err != nil {
		t.Fatal(err)
	}
	c := &core.Core{
		Store: st, SSH: ssh.NewClient(),
		Schedule: schedule.NewManager(
			&broadcast.Runner{Exec: nil, Store: st},
			&lifecycle.Runner{SSH: nil, Store: st},
			st, nil),
	}
	var stdout, stderr bytes.Buffer
	if err := shutdownCmd(context.Background(), c, []string{"vm-a", "status"}, &stdout, &stderr); err != nil {
		t.Errorf("status with record: unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "pending") {
		t.Errorf("expected 'pending' in output, got: %q", out)
	}
	if !strings.Contains(out, "vm-a") {
		t.Errorf("expected host 'vm-a' in output, got: %q", out)
	}
}
