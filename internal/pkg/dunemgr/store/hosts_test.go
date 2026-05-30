package store

import (
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"
)

func TestHostPutGet(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "state.bolt"))
	defer s.Close()

	want := HostProfile{
		Name:     "vm-a",
		SSHAlias: "vm-a",
	}
	if err := s.PutHost(want); err != nil {
		t.Fatalf("PutHost: %v", err)
	}
	got, err := s.GetHost("vm-a")
	if err != nil {
		t.Fatalf("GetHost: %v", err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestListHostsEmpty(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "state.bolt"))
	defer s.Close()
	got, err := s.ListHosts()
	if err != nil {
		t.Fatalf("ListHosts: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ListHosts empty store = %v, want []", got)
	}
}

func TestListHostsSorted(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "state.bolt"))
	defer s.Close()
	_ = s.PutHost(HostProfile{Name: "vm-c"})
	_ = s.PutHost(HostProfile{Name: "vm-a"})
	_ = s.PutHost(HostProfile{Name: "vm-b"})
	got, _ := s.ListHosts()
	want := []string{"vm-a", "vm-b", "vm-c"}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	for i, h := range got {
		if h.Name != want[i] {
			t.Errorf("[%d].Name = %q, want %q", i, h.Name, want[i])
		}
	}
}

func TestDeleteHost(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "state.bolt"))
	defer s.Close()
	_ = s.PutHost(HostProfile{Name: "vm-a"})
	if err := s.DeleteHost("vm-a"); err != nil {
		t.Fatalf("DeleteHost: %v", err)
	}
	_, err := s.GetHost("vm-a")
	if err == nil {
		t.Errorf("GetHost after Delete: want error, got nil")
	}
}

func TestGetHostIgnoresLegacyFields(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "state.bolt"))
	defer s.Close()
	// Simulate an old record with extra keys by storing raw JSON.
	raw := `{"name":"vm-a","ssh_alias":"vm-a","fqdn":"x","k3s_ca_b64":"y"}`
	if err := s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte("hosts")).Put([]byte("vm-a"), []byte(raw))
	}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetHost("vm-a")
	if err != nil || got.Name != "vm-a" || got.SSHAlias != "vm-a" {
		t.Errorf("got=%+v err=%v, want vm-a/vm-a", got, err)
	}
}
