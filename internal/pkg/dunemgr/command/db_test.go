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

func TestDBUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := dbCmd(context.Background(), nil, []string{}, &stdout, &stderr); err == nil {
		t.Error("db no args: err=nil, want non-nil")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Errorf("missing usage hint: %q", stderr.String())
	}
}

func TestDBRejectsUnknownSubcommand(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &core.Core{Store: st, SSH: ssh.NewClient()}
	var stdout, stderr bytes.Buffer
	if err := dbCmd(context.Background(), c, []string{"vm-a", "frobnicate"}, &stdout, &stderr); err == nil {
		t.Error("db bogus sub: err=nil, want non-nil")
	}
}

func TestDBExecRequiresSQL(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &core.Core{Store: st, SSH: ssh.NewClient()}
	var stdout, stderr bytes.Buffer
	if err := dbCmd(context.Background(), c, []string{"vm-a", "exec"}, &stdout, &stderr); err == nil {
		t.Error("exec without sql: err=nil, want non-nil")
	}
}

func TestDBColumnsRequiresArgs(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &core.Core{Store: st, SSH: ssh.NewClient()}
	var stdout, stderr bytes.Buffer
	if err := dbCmd(context.Background(), c, []string{"vm-a", "columns", "dune"}, &stdout, &stderr); err == nil {
		t.Error("columns with missing table: err=nil, want non-nil")
	}
}
