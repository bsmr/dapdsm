package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/steamcmd"
)

// fakeSteamRunner returns canned stdout for cat (the version.txt read).
type fakeSteamRunner struct {
	calls [][]string
}

func (f *fakeSteamRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	if name == "cat" {
		return "v1.5.0\n", nil
	}
	return "", nil
}

func TestDepotCmd_Acquire(t *testing.T) {
	f := &fakeSteamRunner{}
	nr := func(string) steamcmd.Runner { return f }
	var out, errOut bytes.Buffer
	err := depotCmd(context.Background(), nr, []string{"acquire", "--jump", "j", "--env", "prod"}, &out, &errOut)
	if err != nil {
		t.Fatalf("depotCmd: %v", err)
	}
	if !strings.Contains(out.String(), "v1.5.0") {
		t.Errorf("output missing version: %q", out.String())
	}
	// default staging path for prod + the prod appID must have been used.
	joined := ""
	for _, c := range f.calls {
		joined += strings.Join(c, " ") + "\n"
	}
	if !strings.Contains(joined, "/home/dune/depot/prod") {
		t.Errorf("default staging path not used:\n%s", joined)
	}
	if !strings.Contains(joined, "+app_update 4754530") {
		t.Errorf("prod appID not used:\n%s", joined)
	}
}

func TestDepotCmd_MissingFlags(t *testing.T) {
	nr := func(string) steamcmd.Runner {
		t.Fatal("runner factory must not be called on a usage error")
		return nil
	}
	var out, errOut bytes.Buffer
	if err := depotCmd(context.Background(), nr, []string{"acquire", "--jump", "j"}, &out, &errOut); err == nil {
		t.Fatal("expected error when --env is missing")
	}
}

func TestDepotCmd_BadEnv(t *testing.T) {
	nr := func(string) steamcmd.Runner { return &fakeSteamRunner{} }
	var out, errOut bytes.Buffer
	if err := depotCmd(context.Background(), nr, []string{"acquire", "--jump", "j", "--env", "staging"}, &out, &errOut); err == nil {
		t.Fatal("expected error for unknown env")
	}
}

var _ steamcmd.Runner = jumpRunner{}
