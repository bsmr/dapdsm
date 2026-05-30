package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func runArgs(args ...string) (string, string, error) {
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
	_, _, err := runArgs("frobnicate")
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
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), []string{"serve", "--bogus"}, nil, &stdout, &stderr)
	if err == nil {
		t.Error("serve --bogus: want parse error, got nil")
	}
}
