package store

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestExportRecordRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "dunemgr.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	rec := ExportRecord{
		Host:          "vm-dune-01",
		FLSID:         "abc123",
		CharacterName: "Muad'Dib",
		UnixTS:        1700000000,
		LocalPath:     "/data/avatars/vm-dune-01/abc123-1700000000.json",
		Bytes:         42,
		Checksum:      "deadbeef",
		Operator:      "cli",
	}
	if err := s.PutExport(rec); err != nil {
		t.Fatalf("PutExport: %v", err)
	}
	got, err := s.GetExport(rec.Key())
	if err != nil {
		t.Fatalf("GetExport: %v", err)
	}
	if got.FLSID != "abc123" || got.CharacterName != "Muad'Dib" || got.Checksum != "deadbeef" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("CreatedAt should be set by PutExport")
	}
}

func TestListExportsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "dunemgr.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	for _, ts := range []int64{100, 300, 200} {
		if err := s.PutExport(ExportRecord{Host: "h", FLSID: "f", UnixTS: ts}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.PutExport(ExportRecord{Host: "other", FLSID: "f", UnixTS: 999}); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListExports("h")
	if err != nil {
		t.Fatalf("ListExports: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows for host h, got %d", len(rows))
	}
	if rows[0].UnixTS != 300 || rows[2].UnixTS != 100 {
		t.Fatalf("not newest-first: %d..%d", rows[0].UnixTS, rows[2].UnixTS)
	}
}

func TestGetExportNotFound(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "dunemgr.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if _, err := s.GetExport("nope"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestExportKeyIsPrintableAndRoundTrips(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rec := ExportRecord{Host: "vm-a", FLSID: "DEADBEEF", UnixTS: 42, CharacterName: "Mal"}
	if strings.ContainsRune(rec.Key(), '\x00') {
		t.Fatalf("key must not contain NUL: %q", rec.Key())
	}
	if err := s.PutExport(rec); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetExport(rec.Key())
	if err != nil || got.CharacterName != "Mal" {
		t.Fatalf("round-trip got=%+v err=%v", got, err)
	}
	rows, err := s.ListExports("vm-a")
	if err != nil || len(rows) != 1 {
		t.Fatalf("ListExports got=%d err=%v, want 1", len(rows), err)
	}
}
