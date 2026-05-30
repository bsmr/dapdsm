package dbquery

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

type pipingRunner struct {
	stdinSeen []byte
	argsSeen  []string
	mqStdout  string
	podStdout string
}

func (r *pipingRunner) Run(ctx context.Context, name string, args ...string) (ssh.Result, error) {
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "get databasedeployment") {
		return ssh.Result{Stdout: r.podStdout, ExitCode: 0}, nil
	}
	return ssh.Result{ExitCode: 0}, nil
}

func (r *pipingRunner) RunWithStdin(ctx context.Context, stdin []byte, name string, args ...string) (ssh.Result, error) {
	r.stdinSeen = append([]byte(nil), stdin...)
	r.argsSeen = append([]string{name}, args...)
	return ssh.Result{Stdout: r.mqStdout, ExitCode: 0}, nil
}

func newTempStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "dunemgr.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestExecPipesSQLAndRecordsAudit(t *testing.T) {
	rr := &pipingRunner{
		podStdout: fakeDBDeploy,
		mqStdout:  "id|name\n1|test\n",
	}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}

	res, err := r.Exec(context.Background(), "operator", "vm-a", "SELECT id, name FROM dune.player")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if !bytes.Equal(rr.stdinSeen, []byte("SELECT id, name FROM dune.player")) {
		t.Errorf("stdin=%q", rr.stdinSeen)
	}
	if !strings.Contains(res.Stdout, "test") {
		t.Errorf("res.Stdout missing result: %q", res.Stdout)
	}
	// Regression: psql must connect over TCP on the discovered port/user, in
	// the discovered namespace — this is the bug this change fixes.
	argv := strings.Join(rr.argsSeen, " ")
	for _, want := range []string{"-n funcom-seabass-x", "-h 127.0.0.1", "-p 15432", "-U postgres", "-d dune"} {
		if !strings.Contains(argv, want) {
			t.Errorf("psql argv %q missing %q", argv, want)
		}
	}

	entries, _ := r.Store.ListAudit(0)
	if len(entries) != 1 || entries[0].Action != "db.exec" || entries[0].Result != "ok" {
		t.Errorf("audit=%+v", entries)
	}
}

func TestExecRecordsErrorOnDiscoveryFailure(t *testing.T) {
	rr := &pipingRunner{podStdout: ""}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	if _, err := r.Exec(context.Background(), "operator", "vm-a", "SELECT 1"); err == nil {
		t.Error("Exec with empty discovery: err=nil, want non-nil")
	}
	entries, _ := r.Store.ListAudit(0)
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Result, "error:") {
		t.Errorf("audit=%+v", entries)
	}
}
