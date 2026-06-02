package avatar

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
)

func TestImportRequiresConfirm(t *testing.T) {
	db := &fakeDB{offline: map[string]bool{"dst": true}, checksum: map[string]string{"dst": "cs"}}
	r := newRunner(t, db)
	rec, _ := seedExport(t, r, "dst", "fls-9", "cs", "Paul")
	if _, err := r.Import(context.Background(), "cli", "dst", "fls-9", rec.Key(), "", false); err == nil {
		t.Fatal("want error without --confirm")
	}
}

func TestImportConfirmedUsesRecordName(t *testing.T) {
	db := &fakeDB{offline: map[string]bool{"dst": true}, checksum: map[string]string{"dst": "cs"}, importID: 555}
	r := newRunner(t, db)
	rec, _ := seedExport(t, r, "dst", "fls-9", "cs", "Paul")
	id, err := r.Import(context.Background(), "cli", "dst", "fls-9", rec.Key(), "", true)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if id != 555 {
		t.Fatalf("want 555, got %d", id)
	}
	if db.lastImport.name != "Paul" {
		t.Fatalf("name should default to record's: %q", db.lastImport.name)
	}
}

func TestImportChecksumMismatch(t *testing.T) {
	db := &fakeDB{offline: map[string]bool{"dst": true}, checksum: map[string]string{"dst": "OTHER"}}
	r := newRunner(t, db)
	rec, _ := seedExport(t, r, "dst", "fls-9", "cs", "Paul")
	if _, err := r.Import(context.Background(), "cli", "dst", "fls-9", rec.Key(), "", true); err == nil {
		t.Fatal("want checksum-mismatch error")
	}
}

func TestTransferCheckDoesNotMutate(t *testing.T) {
	db := &fakeDB{
		offline:  map[string]bool{"src": true, "dst": true},
		checksum: map[string]string{"src": "cs", "dst": "cs"},
		name:     "Leto",
	}
	r := newRunner(t, db)
	res, err := r.Transfer(context.Background(), "cli", "src", "dst", "fls-9", "", true /*check*/, false)
	if err != nil {
		t.Fatalf("Transfer --check: %v", err)
	}
	if !res.Report.OK() {
		t.Fatalf("check should pass: %+v", res.Report)
	}
	if db.lastImport.host != "" {
		t.Fatal("--check must not call CharacterImport")
	}
	rows, _ := r.Store.ListExports("src")
	if len(rows) != 0 {
		t.Fatal("--check must not write an export record")
	}
	if db.exportCalls != 0 {
		t.Fatalf("--check must not call CharacterExport, got %d calls", db.exportCalls)
	}
}

func TestTransferRealRunExportsThenImports(t *testing.T) {
	db := &fakeDB{
		offline:  map[string]bool{"src": true, "dst": true},
		checksum: map[string]string{"src": "cs", "dst": "cs"},
		name:     "Leto",
		export:   `{"_patches_checksum":"cs","funcom_id":"fls-9"}`,
		importID: 777,
	}
	r := newRunner(t, db)
	res, err := r.Transfer(context.Background(), "cli", "src", "dst", "fls-9", "", false, true /*confirm*/)
	if err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	if res.ControllerID != 777 {
		t.Fatalf("want 777, got %d", res.ControllerID)
	}
	if db.lastImport.host != "dst" || db.lastImport.fls != "fls-9" || db.lastImport.name != "Leto" {
		t.Fatalf("import not invoked with src name/dst: %+v", db.lastImport)
	}
	if rows, _ := r.Store.ListExports("src"); len(rows) != 1 {
		t.Fatal("real transfer should leave one export record on src")
	}
	if db.exportCalls != 1 {
		t.Fatalf("real transfer must call CharacterExport exactly once, got %d", db.exportCalls)
	}
}

func TestTransferRealRunNeedsConfirm(t *testing.T) {
	db := &fakeDB{
		offline:  map[string]bool{"src": true, "dst": true},
		checksum: map[string]string{"src": "cs", "dst": "cs"},
		name:     "Leto",
		export:   `{"_patches_checksum":"cs"}`,
	}
	r := newRunner(t, db)
	if _, err := r.Transfer(context.Background(), "cli", "src", "dst", "fls-9", "", false, false); err == nil {
		t.Fatal("real transfer without --confirm must error")
	}
}

func TestImportRefusesOnlineTarget(t *testing.T) {
	db := &fakeDB{offline: map[string]bool{"dst": false}, checksum: map[string]string{"dst": "cs"}}
	r := newRunner(t, db)
	rec, _ := seedExport(t, r, "dst", "fls-9", "cs", "Paul")
	if _, err := r.Import(context.Background(), "cli", "dst", "fls-9", rec.Key(), "", true); err == nil {
		t.Fatal("import must refuse an online target")
	}
}

// seedExport writes an export file + record directly (bypassing the DB) for
// import tests.
func seedExport(t *testing.T, r *Runner, host, fls, checksum, name string) (store.ExportRecord, string) {
	t.Helper()
	data := `{"_patches_checksum":"` + checksum + `","funcom_id":"` + fls + `"}`
	dir := filepath.Join(r.DataDir, host)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, fls+"-1.json")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := store.ExportRecord{Host: host, FLSID: fls, CharacterName: name, UnixTS: 1, LocalPath: path, Checksum: checksum}
	if err := r.Store.PutExport(rec); err != nil {
		t.Fatal(err)
	}
	return rec, data
}

// fakeDB implements the DB interface for tests; no live cluster.
type fakeDB struct {
	offline     map[string]bool   // host -> offline?
	checksum    map[string]string // host -> checksum
	name        string
	export      string
	exportCalls int
	importID    int64
	importErr   error
	lastImport  struct{ host, data, fls, name string }
}

func (f *fakeDB) IsPlayerOffline(_ context.Context, host, _ string) (bool, error) {
	return f.offline[host], nil
}
func (f *fakeDB) PatchesChecksum(_ context.Context, host string) (string, error) {
	return f.checksum[host], nil
}
func (f *fakeDB) CharacterName(_ context.Context, _, _ string) (string, error) {
	return f.name, nil
}
func (f *fakeDB) CharacterExport(_ context.Context, _, _ string) (string, error) {
	f.exportCalls++
	return f.export, nil
}
func (f *fakeDB) CharacterImport(_ context.Context, host, data, fls, name string) (int64, error) {
	f.lastImport = struct{ host, data, fls, name string }{host, data, fls, name}
	return f.importID, f.importErr
}

func newRunner(t *testing.T, db *fakeDB) *Runner {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "d"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return &Runner{DB: db, Store: s, DataDir: filepath.Join(t.TempDir(), "avatars")}
}

func TestExportWritesFileAndRecord(t *testing.T) {
	db := &fakeDB{
		offline: map[string]bool{"src": true},
		name:    "Stilgar",
		export:  `{"_patches_checksum":"cs1","funcom_id":"fls-9"}`,
	}
	r := newRunner(t, db)
	rec, err := r.Export(context.Background(), "cli", "src", "fls-9")
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if rec.Checksum != "cs1" {
		t.Fatalf("checksum not parsed from dump: %q", rec.Checksum)
	}
	if rec.CharacterName != "Stilgar" {
		t.Fatalf("name label not captured: %q", rec.CharacterName)
	}
	if _, err := os.Stat(rec.LocalPath); err != nil {
		t.Fatalf("export file not written: %v", err)
	}
	if got, _ := r.Store.GetExport(rec.Key()); got.FLSID != "fls-9" {
		t.Fatalf("record not stored: %+v", got)
	}
}

func TestExportRefusesOnlinePlayer(t *testing.T) {
	db := &fakeDB{offline: map[string]bool{"src": false}, export: `{}`}
	r := newRunner(t, db)
	if _, err := r.Export(context.Background(), "cli", "src", "fls-9"); err == nil {
		t.Fatal("want error for online player")
	}
}

func TestCheckAllGates(t *testing.T) {
	db := &fakeDB{
		offline:  map[string]bool{"src": true, "dst": true},
		checksum: map[string]string{"src": "cs", "dst": "cs"},
		name:     "Leto",
	}
	r := newRunner(t, db)
	rep, err := r.Check(context.Background(), "src", "dst", "fls-9")
	if err != nil {
		t.Fatal(err)
	}
	if !rep.OK() {
		t.Fatalf("want OK, got %+v", rep)
	}
	if rep.CharacterName != "Leto" {
		t.Fatalf("name not in report: %q", rep.CharacterName)
	}
}

func TestCheckChecksumMismatch(t *testing.T) {
	db := &fakeDB{
		offline:  map[string]bool{"src": true, "dst": true},
		checksum: map[string]string{"src": "cs1", "dst": "cs2"},
		name:     "Leto",
	}
	r := newRunner(t, db)
	rep, err := r.Check(context.Background(), "src", "dst", "fls-9")
	if err != nil {
		t.Fatal(err)
	}
	if rep.OK() || rep.ChecksumMatch {
		t.Fatalf("want checksum mismatch -> not OK, got %+v", rep)
	}
}

func TestCheckFLSMissing(t *testing.T) {
	db := &fakeDB{
		offline:  map[string]bool{"src": true, "dst": true},
		checksum: map[string]string{"src": "cs", "dst": "cs"},
		name:     "", // no such player on src
	}
	r := newRunner(t, db)
	rep, err := r.Check(context.Background(), "src", "dst", "fls-9")
	if err != nil {
		t.Fatal(err)
	}
	if rep.FLSExists || rep.OK() {
		t.Fatalf("empty name must set FLSExists=false and OK=false: %+v", rep)
	}
}
