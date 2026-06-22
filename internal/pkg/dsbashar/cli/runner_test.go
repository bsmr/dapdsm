package cli

import (
	"bytes"
	"context"
	"io"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/clusteraccess"
	"go.muehmer.eu/dapdsm/pkg/transport/kube"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

type stubExecer struct{}

func (stubExecer) Run(context.Context, string, string, ...string) (ssh.Result, error) {
	return ssh.Result{}, nil
}
func (stubExecer) RunWithStdin(context.Context, string, []byte, string, ...string) (ssh.Result, error) {
	return ssh.Result{}, nil
}

func TestNewKubeRunner_DefaultsToLocalCmdRunner(t *testing.T) {
	kubeRunnerFor = func(stderr io.Writer) kube.Runner { return &kube.CmdRunner{Stderr: stderr} }
	if _, ok := newKubeRunner(io.Discard).(*kube.CmdRunner); !ok {
		t.Fatal("without --jump, newKubeRunner must return *kube.CmdRunner")
	}
}

func TestConfigureRunner_JumpSelectsKubeaccess(t *testing.T) {
	prevAccess := resolvedAccess
	t.Cleanup(func() {
		kubeRunnerFor = func(stderr io.Writer) kube.Runner { return &kube.CmdRunner{Stderr: stderr} }
		resolvedAccess = prevAccess
	})
	configureRunner(stubExecer{}, "jh", "/home/dune/kubeconfig")
	if _, ok := newKubeRunner(io.Discard).(*kube.CmdRunner); ok {
		t.Fatal("with --jump set, newKubeRunner must NOT return the local *kube.CmdRunner")
	}
}

func TestRun_ParsesLeadingGlobalsThenDispatches(t *testing.T) {
	prevAccess := resolvedAccess
	prevRunner := kubeRunnerFor
	t.Cleanup(func() {
		resolvedAccess = prevAccess
		kubeRunnerFor = prevRunner
	})
	var out bytes.Buffer
	// `version` needs no cluster; assert global flags are consumed before dispatch.
	err := Run(context.Background(), []string{"--jump", "jh", "--kubeconfig", "/k", "version"},
		stubExecer{}, nil, &out, io.Discard)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("version produced no output — global flag parsing likely swallowed the verb")
	}
	_ = clusteraccess.Descriptor{} // keep import if unused elsewhere
}
