package command

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

func TestPlayerNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := playerCmd(context.Background(), nil, []string{}, &stdout, &stderr); err == nil {
		t.Error("player no args: err=nil, want non-nil")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Errorf("missing usage hint: %q", stderr.String())
	}
}

func TestPlayerMissingSubVerb(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// Only host provided, no sub-verb.
	if err := playerCmd(context.Background(), nil, []string{"vm-a"}, &stdout, &stderr); err == nil {
		t.Error("player host only: err=nil, want non-nil")
	}
}

func TestPlayerUnknownSubVerb(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &core.Core{Store: st, SSH: ssh.NewClient()}
	var stdout, stderr bytes.Buffer
	if err := playerCmd(context.Background(), c, []string{"vm-a", "frobnicate"}, &stdout, &stderr); err == nil {
		t.Error("player unknown sub: err=nil, want non-nil")
	}
}

func TestPlayerSearchRequiresQuery(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &core.Core{Store: st, SSH: ssh.NewClient()}
	var stdout, stderr bytes.Buffer
	// "search" without a query arg → usage error (we require at least the query).
	// Note: we do accept empty string to mean "all", so this tests truly missing arg.
	if err := playerCmd(context.Background(), c, []string{"vm-a", "search"}, &stdout, &stderr); err == nil {
		t.Error("player search no query: err=nil, want non-nil")
	}
}

func TestPlayerPosRequiresFLSID(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &core.Core{Store: st, SSH: ssh.NewClient()}
	var stdout, stderr bytes.Buffer
	if err := playerCmd(context.Background(), c, []string{"vm-a", "pos"}, &stdout, &stderr); err == nil {
		t.Error("player pos no fls-id: err=nil, want non-nil")
	}
}

func TestPlayerRegisteredInDispatchTable(t *testing.T) {
	if !Known("player") {
		t.Error(`"player" verb not registered in dispatch table`)
	}
}

func TestPlayerInspectUsage(t *testing.T) {
	var out, errb bytes.Buffer
	err := Dispatch(context.Background(), &core.Core{}, []string{"player", "h", "inspect"}, &out, &errb)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("inspect without fls should be ErrUsage, got %v", err)
	}
}

func TestPlayerSpecHasInspect(t *testing.T) {
	s, ok := SpecFor("player")
	if !ok {
		t.Fatal("no player spec")
	}
	found := false
	for _, o := range s.Args[1].options {
		if o == "inspect" {
			found = true
		}
	}
	if !found {
		t.Fatalf("player spec sub-verbs missing inspect: %v", s.Args[1].options)
	}
}
