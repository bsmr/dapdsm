package hostpool

import (
	"context"
	"path/filepath"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
)

func newTestManager(t *testing.T) (*Manager, *store.Store) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	m := &Manager{
		Store: s,
	}
	return m, s
}

func TestRegisterStoresProfile(t *testing.T) {
	m, s := newTestManager(t)
	if err := m.Register(context.Background(), "vm-a", "vm-a"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, err := s.GetHost("vm-a")
	if err != nil {
		t.Fatalf("GetHost: %v", err)
	}
	if got.Name != "vm-a" || got.SSHAlias != "vm-a" {
		t.Errorf("profile = %+v, want vm-a/vm-a", got)
	}
}

func TestRegisterRejectsInvalidName(t *testing.T) {
	m, _ := newTestManager(t)
	if err := m.Register(context.Background(), "-evil", "vm-a"); err == nil {
		t.Errorf("Register with invalid name: want error, got nil")
	}
}

func TestListAndDelete(t *testing.T) {
	m, _ := newTestManager(t)
	if err := m.Register(context.Background(), "vm-a", "vm-a"); err != nil {
		t.Fatal(err)
	}
	if err := m.Register(context.Background(), "vm-b", "vm-b"); err != nil {
		t.Fatal(err)
	}
	all, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("len = %d, want 2", len(all))
	}
	if err := m.Delete("vm-a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	all, _ = m.List()
	if len(all) != 1 || all[0].Name != "vm-b" {
		t.Errorf("after delete: %+v", all)
	}
}
