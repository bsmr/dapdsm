package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// runArgs invokes Run with no XDG isolation; use for verbs that do not open
// the core (help, version, serve --bogus, etc.).
func runArgs(args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), args, nil, &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

// runArgsXDG invokes Run with a temp XDG environment so core.Open does not
// touch the real ~/.config or ~/.local/share. Use for verbs that fall through
// to the core+dispatcher path (unknown subcommands, host-targeting verbs).
func runArgsXDG(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), args, nil, &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

func TestRunHelpPrintsUsage(t *testing.T) {
	out, _, err := runArgs("help")
	if err != nil {
		t.Fatalf("help: %v", err)
	}
	if !strings.Contains(out, "dunemgr") || !strings.Contains(out, "Commands:") {
		t.Errorf("help output = %q", out)
	}
}

func TestRunUnknownSubcommandIsErrUsage(t *testing.T) {
	_, _, err := runArgsXDG(t, "frobnicate")
	if !errors.Is(err, ErrUsage) {
		t.Errorf("unknown subcommand err = %v, want ErrUsage", err)
	}
}

func TestRunVersionPrintsVersion(t *testing.T) {
	out, _, err := runArgs("version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if !strings.Contains(out, "dunemgr") {
		t.Errorf("version output = %q", out)
	}
}

func TestServeRejectsUnknownFlag(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_DATA_HOME", dir)
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), []string{"serve", "--bogus"}, nil, &stdout, &stderr)
	if err == nil {
		t.Error("serve --bogus: want parse error, got nil")
	}
}
