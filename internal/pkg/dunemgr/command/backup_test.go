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

func TestBackupUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if err := backupCmd(context.Background(), nil, nil, &out, &errb); err == nil {
		t.Error("backup no args: err=nil, want non-nil")
	}
	if !strings.Contains(errb.String(), "usage") {
		t.Errorf("missing usage hint: %q", errb.String())
	}
}

func TestBackupRejectsUnknownSubcommand(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &core.Core{Store: st, SSH: ssh.NewClient(), BackupDir: filepath.Join(t.TempDir(), "backups")}
	var out, errb bytes.Buffer
	if err := backupCmd(context.Background(), c, []string{"vm-a", "bg", "frobnicate"}, &out, &errb); err == nil {
		t.Error("backup bogus sub: err=nil, want non-nil")
	}
}

func TestBackupCreateRequiresName(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &core.Core{Store: st, SSH: ssh.NewClient(), BackupDir: filepath.Join(t.TempDir(), "backups")}
	var out, errb bytes.Buffer
	if err := backupCmd(context.Background(), c, []string{"vm-a", "bg", "create"}, &out, &errb); err == nil {
		t.Error("create without name: err=nil, want non-nil")
	}
}

func TestBackupRestoreRequiresKeyAndConfirm(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &core.Core{Store: st, SSH: ssh.NewClient(), BackupDir: filepath.Join(t.TempDir(), "backups")}
	var out, errb bytes.Buffer
	if err := backupCmd(context.Background(), c, []string{"vm-a", "bg", "restore"}, &out, &errb); err == nil {
		t.Error("restore without key: err=nil, want non-nil")
	}
	if err := backupCmd(context.Background(), c, []string{"vm-a", "bg", "restore", "key"}, &out, &errb); err == nil {
		t.Error("restore without --confirm: err=nil, want non-nil")
	}
}
