package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnv_PrintsParsedConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "dunectl.env")
	body := "TARGET=prod\n" +
		"K3S_EXTRA_TLS_SAN=\"vm-a.example vm-b.example\"\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var stdout, stderr bytes.Buffer
	if err := envCmd(context.Background(), []string{"--config", path}, &stdout, &stderr); err != nil {
		t.Fatalf("envCmd err = %v", err)
	}
	out := stdout.String()
	for _, want := range []string{
		"TARGET:            prod",
		"FLS_TOKEN_FILE:    /etc/dune/fls-token-prod",
		"K3S_EXTRA_TLS_SAN: vm-a.example vm-b.example",
		"(derived) AppID:   4754530",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\nstdout=%s", want, out)
		}
	}
}

func TestEnv_NoExtraSANsPrintsNone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "dunectl.env")
	if err := os.WriteFile(path, []byte("TARGET=test\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var stdout, stderr bytes.Buffer
	if err := envCmd(context.Background(), []string{"--config", path}, &stdout, &stderr); err != nil {
		t.Fatalf("envCmd err = %v", err)
	}
	if !strings.Contains(stdout.String(), "K3S_EXTRA_TLS_SAN: (none)") {
		t.Errorf("stdout = %q, want '(none)'", stdout.String())
	}
	if !strings.Contains(stdout.String(), "(derived) AppID:   3104830") {
		t.Errorf("stdout = %q, want test AppID 3104830", stdout.String())
	}
}

func TestEnv_MissingFileReturnsError(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	err := envCmd(context.Background(), []string{"--config", filepath.Join(t.TempDir(), "no.env")}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("err = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "open ") {
		t.Errorf("err = %v, want substring 'open '", err)
	}
}

func TestEnv_RejectsUnknownFlag(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	err := envCmd(context.Background(), []string{"--no-such-flag"}, &stdout, &stderr)
	if !errors.Is(err, ErrUsage) {
		t.Errorf("err = %v, want errors.Is(err, ErrUsage)", err)
	}
}
