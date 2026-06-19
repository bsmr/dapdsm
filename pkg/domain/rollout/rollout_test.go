package rollout

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/config"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

type recRunner struct {
	calls   []string
	keygenY string // stdout for `age-keygen -y` (recipient)
}

func (r *recRunner) Run(_ context.Context, name string, args ...string) (ssh.Result, error) {
	c := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, c)
	if strings.Contains(c, "age-keygen -y") {
		return ssh.Result{Stdout: r.keygenY}, nil
	}
	return ssh.Result{}, nil
}
func (r *recRunner) RunWithStdin(_ context.Context, _ []byte, name string, args ...string) (ssh.Result, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	return ssh.Result{}, nil
}

func writeSealed(t *testing.T, dir, rel string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("SEALEDBLOB"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRunOrdersStepsAndMapsSecret(t *testing.T) {
	dir := t.TempDir()
	writeSealed(t, dir, "secrets/vm-a/fls-token.age")
	cfg := &config.Config{Targets: []config.Target{{
		Name: "vm-a", Recipient: "age1recip", Kind: "prod",
		Binaries: []string{"ds-bashar"},
		Secrets:  map[string]string{"fls-token": "secrets/vm-a/fls-token.age"},
	}}}
	rr := &recRunner{keygenY: "age1recip\n"}
	var built []string
	deps := Deps{
		SSH:        &ssh.Client{Runner: rr},
		Build:      func(_ context.Context, b string) (string, error) { built = append(built, b); return "/bin/" + b, nil },
		EtcArchive: func(_ context.Context) ([]byte, error) { return []byte("TAR"), nil },
		Stdout:     &bytes.Buffer{},
	}
	if err := Run(context.Background(), deps, cfg, dir, "vm-a"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(built) != 1 || built[0] != "ds-bashar" {
		t.Fatalf("built = %v", built)
	}
	joined := strings.Join(rr.calls, " || ")
	// fls-token maps to /etc/dune/fls-token-prod (kind=prod), and an age -d unseal happened.
	if !strings.Contains(joined, "/etc/dune/fls-token-prod") || !strings.Contains(joined, "age -d") {
		t.Fatalf("secret not unsealed to mapped path: %s", joined)
	}
	if !strings.Contains(joined, "install") || !strings.Contains(joined, "/usr/local/bin/ds-bashar") {
		t.Fatalf("binary not installed: %s", joined)
	}
}

func TestRunRejectsRecipientMismatch(t *testing.T) {
	cfg := &config.Config{Targets: []config.Target{{Name: "vm-a", Recipient: "age1expected", Binaries: []string{"ds-bashar"}}}}
	rr := &recRunner{keygenY: "age1different\n"}
	deps := Deps{SSH: &ssh.Client{Runner: rr},
		Build:      func(_ context.Context, b string) (string, error) { return "/bin/" + b, nil },
		EtcArchive: func(_ context.Context) ([]byte, error) { return nil, nil }, Stdout: &bytes.Buffer{}}
	if err := Run(context.Background(), deps, cfg, t.TempDir(), "vm-a"); err == nil {
		t.Fatal("want error on recipient mismatch, got nil")
	}
}

func TestRunUnknownTarget(t *testing.T) {
	if err := Run(context.Background(), Deps{}, &config.Config{}, t.TempDir(), "nope"); err == nil {
		t.Fatal("want error for unknown target")
	}
}
