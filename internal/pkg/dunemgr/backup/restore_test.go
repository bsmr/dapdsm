package backup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

type restoreFake struct {
	imported   bool
	scpUploads []string
}

func (f *restoreFake) Run(ctx context.Context, name string, args ...string) (ssh.Result, error) {
	joined := name + " " + strings.Join(args, " ")
	if name == "ssh" {
		// After the shell-quoting fix the binary path and "import" verb are
		// individually quoted: e.g. "'/home/dune/.dune/bin/battlegroup' 'import'".
		if strings.Contains(joined, "battlegroup") && strings.Contains(joined, "'import'") {
			f.imported = true
			return ssh.Result{Stdout: "imported\n", ExitCode: 0}, nil
		}
	}
	if name == "scp" && len(args) >= 2 {
		f.scpUploads = append(f.scpUploads, args[len(args)-1])
	}
	return ssh.Result{ExitCode: 0}, nil
}

func (f *restoreFake) RunWithStdin(ctx context.Context, stdin []byte, name string, args ...string) (ssh.Result, error) {
	return ssh.Result{}, nil
}

func writeBackupPair(t *testing.T, dataDir, host, bg, name string, unixTS int64) store.BackupRecord {
	t.Helper()
	dir := filepath.Join(dataDir, host, bg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	bp := filepath.Join(dir, "X.backup")
	if err := os.WriteFile(bp, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write backup: %v", err)
	}
	if err := os.WriteFile(bp+".yaml", []byte("yaml: x"), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	return store.BackupRecord{
		Host: host, BG: bg, Name: name, UnixTS: unixTS,
		LocalPath: bp,
		CreatedAt: time.Unix(unixTS, 0).UTC(),
	}
}

func TestRestoreRefusesWithoutConfirm(t *testing.T) {
	st := newStore(t)
	rec := writeBackupPair(t, t.TempDir(), "vm-a", "bg", "weekly", 1717000000)
	_ = st.PutBackup(rec)
	r := &Runner{SSH: &ssh.Client{Runner: &restoreFake{}}, Store: st, DataDir: filepath.Dir(filepath.Dir(rec.LocalPath))}
	if err := r.Restore(context.Background(), "operator", rec.Key(), false); err == nil {
		t.Error("Restore confirm=false err=nil, want non-nil")
	}
}

func TestRestoreHappyPath(t *testing.T) {
	dataDir := t.TempDir()
	st := newStore(t)
	rec := writeBackupPair(t, dataDir, "vm-a", "bg", "weekly", 1717000000)
	_ = st.PutBackup(rec)
	fake := &restoreFake{}
	r := &Runner{SSH: &ssh.Client{Runner: fake}, Store: st, DataDir: dataDir}

	if err := r.Restore(context.Background(), "operator", rec.Key(), true); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if !fake.imported {
		t.Error("battlegroup import never invoked")
	}
	if len(fake.scpUploads) != 2 {
		t.Errorf("scp upload count=%d, want 2", len(fake.scpUploads))
	}

	entries, _ := st.ListAudit(0)
	if len(entries) != 1 || entries[0].Action != "backup.restore" || entries[0].Result != "ok" {
		t.Errorf("audit=%+v", entries)
	}
}

func TestRestoreMissingRecord(t *testing.T) {
	r := &Runner{SSH: &ssh.Client{Runner: &restoreFake{}}, Store: newStore(t), DataDir: t.TempDir()}
	if err := r.Restore(context.Background(), "operator", "doesnt-exist", true); err == nil {
		t.Error("Restore missing record err=nil, want non-nil")
	}
}
