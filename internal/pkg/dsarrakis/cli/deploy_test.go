package cli

import (
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

type fakeBuilder struct{ built string }

func (f *fakeBuilder) Build(_ context.Context, out string) error { f.built = out; return nil }

type fakeSSH struct {
	sent [3]string
	ran  []string
}

func (f *fakeSSH) SendFile(_ context.Context, host, local, remote string) error {
	f.sent = [3]string{host, local, remote}
	return nil
}
func (f *fakeSSH) Run(_ context.Context, host, cmd string, args ...string) (ssh.Result, error) {
	f.ran = append([]string{host, cmd}, args...)
	return ssh.Result{}, nil
}

func TestDeploy_BuildsShipsInvokes(t *testing.T) {
	b := &fakeBuilder{}
	s := &fakeSSH{}
	err := deploy(context.Background(), s, b, "vm-host", []string{"--distro", "k3s"}, &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	if b.built == "" {
		t.Fatal("binary not built")
	}
	if s.sent[0] != "vm-host" {
		t.Fatalf("sent to wrong host: %v", s.sent)
	}
	joined := strings.Join(s.ran, " ")
	if !strings.Contains(joined, "ds-arrakis host") || !strings.Contains(joined, "--distro k3s") {
		t.Fatalf("invoke wrong: %s", joined)
	}
}
