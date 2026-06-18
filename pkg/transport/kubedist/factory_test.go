package kubedist

import (
	"io"
	"testing"
)

func TestNew(t *testing.T) {
	for _, name := range []string{"k3s", "rke2"} {
		d, err := New(name, &FakeRunner{}, io.Discard)
		if err != nil || d.Name() != name {
			t.Fatalf("New(%q): %v / %v", name, err, d)
		}
	}
	if _, err := New("microk8s", &FakeRunner{}, io.Discard); err == nil {
		t.Fatal("expected error for unknown distro")
	}
}
