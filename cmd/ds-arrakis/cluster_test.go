package main

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/clusteraccess"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

type fakeClusterExecer struct {
	stdoutByCmd map[string]string
	lastKubectl []string
}

func (f *fakeClusterExecer) Run(_ context.Context, host, cmd string, args ...string) (ssh.Result, error) {
	// "cat" is clusteraccess.Load reading the inventory file on the jumphost.
	if cmd == "cat" {
		return ssh.Result{Stdout: f.stdoutByCmd["cat"]}, nil
	}
	// "env" is Access.Kubectl exporting KUBECONFIG before invoking kubectl.
	if cmd == "env" {
		f.lastKubectl = append([]string{host, cmd}, args...)
		return ssh.Result{Stdout: f.stdoutByCmd["kubectl"]}, nil
	}
	return ssh.Result{}, nil
}

const testInv = `
all:
  children:
    worker:
      hosts:
        w-0: {ansible_host: 10.0.0.8}
  vars:
    ansible_user: installer
    ansible_ssh_private_key_file: /k/id
`

func TestClusterCmd_Nodes(t *testing.T) {
	fe := &fakeClusterExecer{stdoutByCmd: map[string]string{
		"cat":     testInv,
		"kubectl": "NAME   STATUS\nw-0    Ready\n",
	}}
	var out, errOut bytes.Buffer
	err := clusterCmd(context.Background(), fe,
		[]string{"nodes", "--jump", "jump", "--kubeconfig", "/k/cfg", "--inventory", "/k/inv.yml"},
		&out, &errOut)
	if err != nil {
		t.Fatalf("clusterCmd: %v", err)
	}
	if !strings.Contains(out.String(), "w-0") {
		t.Errorf("stdout missing node: %q", out.String())
	}
	// Ensure KUBECONFIG was exported for the kubectl call.
	if !slices.Contains(fe.lastKubectl, "KUBECONFIG=/k/cfg") {
		t.Errorf("KUBECONFIG not found in kubectl call args: %v", fe.lastKubectl)
	}
}

func TestClusterCmd_MissingFlags(t *testing.T) {
	var out, errOut bytes.Buffer
	err := clusterCmd(context.Background(), &fakeClusterExecer{}, []string{"nodes"}, &out, &errOut)
	if err == nil {
		t.Fatal("expected error when --jump/--kubeconfig/--inventory are missing")
	}
	if !errors.Is(err, ErrUsage) {
		t.Errorf("want ErrUsage, got %v", err)
	}
}

// compile-time assertion that the real client satisfies Execer.
var _ clusteraccess.Execer = (*ssh.Client)(nil)
