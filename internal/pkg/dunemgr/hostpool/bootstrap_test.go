package hostpool

import (
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

const sampleKubeconfig = `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: TEFTRTY0Q0FEQVRB
    server: https://vm-a.example.org:6443
  name: default
contexts: []
kind: Config
users: []
`

type fakeSSH struct {
	gotHost string
	gotArgs []string
	out     ssh.Result
	err     error
}

func (f *fakeSSH) Run(ctx context.Context, name string, args ...string) (ssh.Result, error) {
	f.gotHost = name
	f.gotArgs = args
	return f.out, f.err
}

func (f *fakeSSH) RunWithStdin(_ context.Context, _ []byte, _ string, _ ...string) (ssh.Result, error) {
	return ssh.Result{}, nil
}

func TestFetchK3sCAExtractsCAAndFQDN(t *testing.T) {
	f := &fakeSSH{out: ssh.Result{Stdout: sampleKubeconfig}}
	client := &ssh.Client{Runner: f}
	ca, fqdn, err := fetchK3sCA(context.Background(), client, "vm-a")
	if err != nil {
		t.Fatalf("fetchK3sCA: %v", err)
	}
	if ca != "TEFTRTY0Q0FEQVRB" {
		t.Errorf("ca = %q, want TEFTRTY0Q0FEQVRB", ca)
	}
	if fqdn != "vm-a.example.org" {
		t.Errorf("fqdn = %q, want vm-a.example.org", fqdn)
	}
	if f.gotHost != "ssh" {
		t.Errorf("invoked %q, want ssh", f.gotHost)
	}
	joined := strings.Join(f.gotArgs, " ")
	if !strings.Contains(joined, "cat /etc/rancher/k3s/k3s.yaml") {
		t.Errorf("ssh args missing cat command: %q", joined)
	}
}

func TestFetchK3sCAMissingCA(t *testing.T) {
	body := "apiVersion: v1\nclusters:\n- cluster:\n    server: https://x:6443\n  name: default\n"
	f := &fakeSSH{out: ssh.Result{Stdout: body}}
	client := &ssh.Client{Runner: f}
	_, _, err := fetchK3sCA(context.Background(), client, "vm-a")
	if err == nil {
		t.Errorf("missing CA: want error, got nil")
	}
}

func TestFetchK3sCASSHError(t *testing.T) {
	f := &fakeSSH{err: ssh.ErrRemoteFailure}
	client := &ssh.Client{Runner: f}
	_, _, err := fetchK3sCA(context.Background(), client, "vm-a")
	if err == nil {
		t.Errorf("ssh error: want error, got nil")
	}
}
