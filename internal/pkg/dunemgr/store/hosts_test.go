package store

import (
	"path/filepath"
	"testing"
)

func TestHostPutGet(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "state.bolt"))
	defer s.Close()

	want := HostProfile{
		Name:        "vm-a",
		SSHAlias:    "vm-a",
		FQDN:        "vm-a.example.org",
		K3sCABase64: "QkFTRTY0Q0E=",
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
