package kubedist

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

var errTest = errors.New("not ready")

func TestK3sInstall_WritesConfigAndRunsInstaller(t *testing.T) {
	f := &FakeRunner{}
	k := newK3s(f, io.Discard)
	err := k.Install(context.Background(), Config{ExternalIP: "1.2.3.4", TLSSANs: []string{"h.example"}})
	if err != nil {
		t.Fatal(err)
	}
	joined := flatten(f.Calls)
	for _, want := range []string{
		"/etc/rancher/k3s/config.yaml",
		"config.yaml.d/20-external-ip.yaml",
		"get.k3s.io",
		"systemctl",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in calls:\n%s", want, joined)
		}
	}
}

func TestK3sName(t *testing.T) {
	if newK3s(&FakeRunner{}, io.Discard).Name() != "k3s" {
		t.Fatal("name")
	}
}

func flatten(calls [][]string) string {
	var b strings.Builder
	for _, c := range calls {
		b.WriteString(strings.Join(c, " "))
		b.WriteString("\n")
	}
	return b.String()
}

func TestEnsureReady_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled
	f := &FakeRunner{Errs: map[string]error{"kubectl": errTest}}
	err := newK3s(f, io.Discard).EnsureReady(ctx)
	if err == nil {
		t.Fatal("want error when context is cancelled and node never ready")
	}
}
