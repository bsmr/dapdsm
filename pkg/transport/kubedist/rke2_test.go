package kubedist

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestRKE2Install_UsesRKE2PathsAndInstaller(t *testing.T) {
	f := &FakeRunner{}
	if err := newRKE2(f, io.Discard).Install(context.Background(), Config{ExternalIP: "1.2.3.4"}); err != nil {
		t.Fatal(err)
	}
	joined := flatten(f.Calls)
	for _, want := range []string{"/etc/rancher/rke2/config.yaml", "get.rke2.io", "rke2-server"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in:\n%s", want, joined)
		}
	}
}

func TestRKE2Name(t *testing.T) {
	if newRKE2(&FakeRunner{}, io.Discard).Name() != "rke2" {
		t.Fatal("name")
	}
}
