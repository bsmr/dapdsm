package command

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/pkg/domain/store"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

func TestHostNoArgsIsUsage(t *testing.T) {
	var out, errb bytes.Buffer
	err := hostCmd(context.Background(), nil, nil, &out, &errb)
	if err == nil {
		t.Error("host with no args: want error, got nil")
	}
}

func TestHostUnknownSubcommand(t *testing.T) {
	var out, errb bytes.Buffer
	err := hostCmd(context.Background(), nil, []string{"nope"}, &out, &errb)
	if err == nil {
		t.Error("host nope: want error, got nil")
	}
}

func TestHostListEmpty(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &core.Core{Store: st, SSH: ssh.NewClient()}

	var out, errb bytes.Buffer
	if err := hostCmd(context.Background(), c, []string{"list"}, &out, &errb); err != nil {
		t.Fatalf("host list (empty): %v", err)
	}
	if !strings.Contains(out.String(), "NAME") {
		t.Errorf("host list output missing header: %q", out.String())
	}
}

func TestHostProbeUnknownHost(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &core.Core{Store: st, SSH: ssh.NewClient()}

	var out, errb bytes.Buffer
	if err := hostCmd(context.Background(), c, []string{"probe", "ghost"}, &out, &errb); err == nil {
		t.Error("probe of unknown host: want error, got nil")
	}
}
