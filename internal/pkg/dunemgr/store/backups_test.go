package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestBackupRecordRoundtrip(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "dunemgr.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	rec := BackupRecord{
		Host:      "vm-a",
		BG:        "myBG",
		Name:      "weekly",
		UnixTS:    1717000000,
		LocalPath: "/tmp/foo.backup",
		Bytes:     12345,
		YAMLBytes: 678,
		Operator:  "local",
		CreatedAt: time.Unix(1717000000, 0).UTC(),
	}
	if err := s.PutBackup(rec); err != nil {
		t.Fatalf("PutBackup: %v", err)
	}

	got, err := s.GetBackup(rec.Key())
	if err != nil {
		t.Fatalf("GetBackup: %v", err)
	}
	if got.Name != rec.Name || got.Bytes != rec.Bytes || got.Host != rec.Host {
		t.Errorf("roundtrip mismatch: got=%+v want=%+v", got, rec)
	}

	rows, err := s.ListBackups("vm-a", "myBG")
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("List len=%d, want 1", len(rows))
	}

	// Different host shouldn't match.
	rows, _ = s.ListBackups("vm-b", "myBG")
	if len(rows) != 0 {
		t.Errorf("cross-host bleed: got %d, want 0", len(rows))
	}

	if err := s.DeleteBackup(rec.Key()); err != nil {
		t.Fatalf("DeleteBackup: %v", err)
	}
	if _, err := s.GetBackup(rec.Key()); err == nil {
		t.Error("GetBackup after delete err=nil, want ErrNotFound")
	}
}

func TestListBackupsSortedNewestFirst(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "dunemgr.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	for _, ts := range []int64{1000, 3000, 2000} {
		_ = s.PutBackup(BackupRecord{
			Host: "vm-a", BG: "bg", Name: "n", UnixTS: ts,
			CreatedAt: time.Unix(ts, 0).UTC(),
		})
	}
	rows, _ := s.ListBackups("vm-a", "bg")
	if len(rows) != 3 {
		t.Fatalf("len=%d, want 3", len(rows))
	}
	if rows[0].UnixTS != 3000 || rows[2].UnixTS != 1000 {
		t.Errorf("sort order: got %v %v %v, want 3000 2000 1000", rows[0].UnixTS, rows[1].UnixTS, rows[2].UnixTS)
	}
}
