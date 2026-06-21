package cli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

func TestDiscoverCmd_ListsBGs(t *testing.T) {
	t.Cleanup(func() { kubeRunnerFor = func(w io.Writer) kube.Runner { return &kube.CmdRunner{Stderr: w} } })
	kubeRunnerFor = func(io.Writer) kube.Runner { return &fakeRunner{nsOut: "funcom-seabass-abc\n"} }
	var out bytes.Buffer
	if err := discoverCmd(context.Background(), nil, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "funcom-seabass-abc") {
		t.Fatalf("missing BG: %s", out.String())
	}
}

func TestDiscoverCmd_EmptyCluster(t *testing.T) {
	t.Cleanup(func() { kubeRunnerFor = func(w io.Writer) kube.Runner { return &kube.CmdRunner{Stderr: w} } })
	kubeRunnerFor = func(io.Writer) kube.Runner { return &fakeRunner{nsOut: "kube-system\n"} }
	var out bytes.Buffer
	if err := discoverCmd(context.Background(), nil, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "no BattleGroups found") {
		t.Fatalf("expected empty-cluster message: %s", out.String())
	}
}
