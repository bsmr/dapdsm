package backup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

type sshFake struct {
	runCalls   []string
	stdinCalls []string
	scpSources []string
	scpTargets []string
	wrapperOK  bool
}

func (f *sshFake) Run(ctx context.Context, name string, args ...string) (ssh.Result, error) {
	joined := name + " " + strings.Join(args, " ")
	f.runCalls = append(f.runCalls, joined)
	if name == "ssh" {
		// After the shell-quoting fix the binary path and "backup" verb are
		// individually quoted: e.g. "'/home/dune/.dune/bin/battlegroup' 'backup'".
		// Match using the shared suffix of the binary name + adjacent verb.
		if strings.Contains(joined, "battlegroup") && strings.Contains(joined, "'backup'") {
			if !f.wrapperOK {
				return ssh.Result{Stderr: "boom", ExitCode: 1}, nil
			}
			return ssh.Result{Stdout: "backup ok\n", ExitCode: 0}, nil
		}
	}
	if name == "scp" {
		// args[-2] is source, args[-1] is destination after `--`
		if len(args) >= 2 {
			f.scpSources = append(f.scpSources, args[len(args)-2])
			f.scpTargets = append(f.scpTargets, args[len(args)-1])
			// pretend SCP created the file
			_ = os.WriteFile(args[len(args)-1], []byte("dummy"), 0o644)
		}
		return ssh.Result{ExitCode: 0}, nil
	}
	return ssh.Result{ExitCode: 0}, nil
}

func (f *sshFake) RunWithStdin(ctx context.Context, stdin []byte, name string, args ...string) (ssh.Result, error) {
	f.stdinCalls = append(f.stdinCalls, name)
	return ssh.Result{}, nil
}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "dunemgr.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestCreateHappyPath(t *testing.T) {
	dataDir := t.TempDir()
	f := &sshFake{wrapperOK: true}
	r := &Runner{
		SSH:     &ssh.Client{Runner: f},
		Store:   newStore(t),
		DataDir: dataDir,
	}

	rec, err := r.Create(context.Background(), "operator", "vm-a", "myBG", "weekly")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.Host != "vm-a" || rec.BG != "myBG" || rec.Name != "weekly" {
		t.Errorf("record fields: %+v", rec)
	}
	if rec.LocalPath == "" {
		t.Error("LocalPath empty")
	}
	if _, err := os.Stat(rec.LocalPath); err != nil {
		t.Errorf("backup file not present: %v", err)
	}
	if _, err := os.Stat(rec.LocalPath + ".yaml"); err != nil {
		t.Errorf("backup.yaml not present: %v", err)
	}

	// audit
	entries, _ := r.Store.ListAudit(0)
	if len(entries) != 1 || entries[0].Action != "backup.create" || entries[0].Result != "ok" {
		t.Errorf("audit=%+v", entries)
	}

	// 2 scp pulls happened with correct sources
	if len(f.scpSources) != 2 {
		t.Fatalf("scp count=%d, want 2", len(f.scpSources))
	}
	if !strings.HasPrefix(f.scpSources[0], "vm-a:/funcom/artifacts/database-dumps/myBG/weekly.backup") {
		t.Errorf("scp source[0]=%q", f.scpSources[0])
	}
}

func TestCreateWrapperFailsRecordsAuditNoRecord(t *testing.T) {
	dataDir := t.TempDir()
	f := &sshFake{wrapperOK: false}
	r := &Runner{
		SSH:     &ssh.Client{Runner: f},
		Store:   newStore(t),
		DataDir: dataDir,
	}

	if _, err := r.Create(context.Background(), "operator", "vm-a", "myBG", "weekly"); err == nil {
		t.Error("Create err=nil, want non-nil")
	}
	rows, _ := r.Store.ListBackups("vm-a", "myBG")
	if len(rows) != 0 {
		t.Errorf("backup row created on failure: %+v", rows)
	}
	entries, _ := r.Store.ListAudit(0)
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Result, "error:") {
		t.Errorf("audit=%+v", entries)
	}
}

func TestCreateRejectsInvalidName(t *testing.T) {
	r := &Runner{SSH: &ssh.Client{Runner: &sshFake{}}, Store: newStore(t), DataDir: t.TempDir()}
	cases := []string{"", "../etc/passwd", "name with space", "name/slash", "-evil"}
	for _, name := range cases {
		if _, err := r.Create(context.Background(), "operator", "vm-a", "myBG", name); err == nil {
			t.Errorf("Create(name=%q) err=nil, want non-nil", name)
		}
	}
}
