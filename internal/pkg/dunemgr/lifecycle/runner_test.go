package lifecycle

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

type fakeRunner struct {
	gotName string
	gotArgs []string
	result  ssh.Result
	err     error
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (ssh.Result, error) {
	f.gotName = name
	f.gotArgs = append([]string(nil), args...)
	return f.result, f.err
}

func (f *fakeRunner) RunWithStdin(_ context.Context, _ []byte, _ string, _ ...string) (ssh.Result, error) {
	return ssh.Result{}, nil
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

func TestRunHappyPath(t *testing.T) {
	fr := &fakeRunner{result: ssh.Result{Stdout: "ok\n", ExitCode: 0}}
	st := newTempStore(t)
	r := &Runner{SSH: &ssh.Client{Runner: fr}, Store: st}

	res, err := r.Run(context.Background(), "operator", "vm-a", ActionStart)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Action != ActionStart || res.Host != "vm-a" {
		t.Errorf("res=%+v", res)
	}
	if fr.gotName != "ssh" {
		t.Errorf("exec name=%q, want ssh", fr.gotName)
	}
	if !strings.Contains(strings.Join(fr.gotArgs, " "), "vm-a /home/dune/.dune/bin/battlegroup start") {
		t.Errorf("args=%v, want trailing 'vm-a /home/dune/.dune/bin/battlegroup start'", fr.gotArgs)
	}

	entries, err := st.ListAudit(0)
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if len(entries) != 1 || entries[0].Action != "lifecycle.start" || entries[0].Result != "ok" {
		t.Errorf("audit=%+v, want 1 entry lifecycle.start ok", entries)
	}
}

func TestRunRecordsErrorOnNonZeroExit(t *testing.T) {
	fr := &fakeRunner{result: ssh.Result{Stderr: "boom", ExitCode: 7}, err: errors.New("exit 7")}
	st := newTempStore(t)
	r := &Runner{SSH: &ssh.Client{Runner: fr}, Store: st}

	if _, err := r.Run(context.Background(), "operator", "vm-a", ActionStop); err == nil {
		t.Fatal("Run err=nil, want non-nil")
	}

	entries, _ := st.ListAudit(0)
	if len(entries) != 1 {
		t.Fatalf("audit len=%d, want 1", len(entries))
	}
	if !strings.HasPrefix(entries[0].Result, "error:") {
		t.Errorf("audit Result=%q, want error: prefix", entries[0].Result)
	}
}

func TestRunRejectsInvalidAction(t *testing.T) {
	r := &Runner{SSH: &ssh.Client{Runner: &fakeRunner{}}, Store: newTempStore(t)}
	if _, err := r.Run(context.Background(), "operator", "vm-a", Action("nuke")); err == nil {
		t.Error("Run with bogus action err=nil, want non-nil")
	}
}
