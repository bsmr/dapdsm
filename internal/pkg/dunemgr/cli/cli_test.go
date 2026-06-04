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

func TestHelpListsAllDispatcherVerbs(t *testing.T) {
	var out bytes.Buffer
	if err := Run(context.Background(), []string{"help"}, nil, &out, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, verb := range []string{"admin", "avatar", "backup", "broadcast", "db", "give", "host", "ini", "item", "lifecycle", "player", "shutdown", "stats", "whisper"} {
		if !strings.Contains(got, verb) {
			t.Fatalf("help missing dispatcher verb %q", verb)
		}
	}
}

// The web UI is disabled in 0.1.12: serve and the token verbs are unwired from
// the CLI, so they are now rejected as unknown verbs (ErrUsage) without opening
// the core. The server/ package and token code stay in the tree for re-enabling.
func TestWebUIVerbsAreUnwired(t *testing.T) {
	for _, verb := range []string{"serve", "--print-token", "regen-token"} {
		_, _, err := runArgs(verb)
		if !errors.Is(err, ErrUsage) {
			t.Errorf("%q: err = %v, want ErrUsage", verb, err)
		}
	}
}

// With the web UI disabled, usage must no longer advertise it or its token
// verbs, and must present the TUI as the default action.
func TestHelpOmitsWebUI(t *testing.T) {
	out, _, err := runArgs("help")
	if err != nil {
		t.Fatalf("help: %v", err)
	}
	for _, gone := range []string{"web UI", "regen-token", "--print-token"} {
		if strings.Contains(out, gone) {
			t.Errorf("help still mentions %q:\n%s", gone, out)
		}
	}
	if !strings.Contains(out, "terminal UI") {
		t.Errorf("help does not present the terminal UI as default:\n%s", out)
	}
}
