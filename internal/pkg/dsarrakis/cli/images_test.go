package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/clusteraccess"
	"go.muehmer.eu/dapdsm/pkg/transport/skopeo"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// imgExecer answers cat (inventory/metadata for Load) + records kubectl runs.
type imgExecer struct {
	runs  [][]string
	stdin [][]byte
}

func (e *imgExecer) Run(_ context.Context, host, cmd string, args ...string) (ssh.Result, error) {
	e.runs = append(e.runs, append([]string{cmd}, args...))
	if cmd == "cat" {
		// minimal inventory: one worker so Load/parseInventory yields >0 nodes
		return ssh.Result{Stdout: imgInventoryYAML}, nil
	}
	return ssh.Result{}, nil
}
func (e *imgExecer) RunWithStdin(_ context.Context, host string, in []byte, cmd string, args ...string) (ssh.Result, error) {
	e.stdin = append(e.stdin, in)
	return ssh.Result{}, nil
}

// imgRunner answers ls/tar so Push enumerates one image.
type imgRunner struct{}

func (imgRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	switch name {
	case "sudo":
		// EnsureInstalled: sudo bash -c <installScript>
		return "", nil
	case "sh":
		if len(args) >= 2 && strings.Contains(args[len(args)-1], "operators") {
			return "/d/images/operators/op.tar\n", nil
		}
		return "", nil
	case "tar":
		return `[{"RepoTags":["funcom/op:v1.5.0"]}]`, nil
	case "skopeo":
		return "", nil
	}
	return "", nil
}

func TestImagesCmd_Distribute(t *testing.T) {
	ex := &imgExecer{}
	nr := func(string) skopeo.Runner { return imgRunner{} }
	var out, errOut bytes.Buffer
	err := imagesCmd(context.Background(), ex, nr, []string{
		"distribute", "--jump", "j", "--kubeconfig", "/kc", "--inventory", "/inv",
		"--env", "prod", "--staging", "/d", "--registry", "10.0.0.9:5000",
		"--storage-class", "local-path", "--lb-ip", "10.0.0.9", "--no-verify",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("imagesCmd: %v\nstderr: %s", err, errOut.String())
	}
	// manifest was applied via stdin (Deploy)
	if len(ex.stdin) == 0 || !strings.Contains(string(ex.stdin[0]), "kind: Namespace") {
		t.Errorf("registry manifest not applied via stdin; stdin calls: %d", len(ex.stdin))
	}
	// report mentions the pushed image
	if !strings.Contains(out.String(), "funcom/op:v1.5.0") {
		t.Errorf("output missing pushed ref: %q", out.String())
	}
}

func TestImagesCmd_MissingFlags(t *testing.T) {
	ex := &imgExecer{}
	nr := func(string) skopeo.Runner { return imgRunner{} }
	var out, errOut bytes.Buffer
	if err := imagesCmd(context.Background(), ex, nr,
		[]string{"distribute", "--jump", "j"}, &out, &errOut); err == nil {
		t.Fatal("expected error when required flags missing")
	}
}

func TestImagesLoadUsage(t *testing.T) {
	var out, errb bytes.Buffer
	// no args after "images" -> usage listing both subcommands
	err := imagesCmd(context.Background(), nil, realImagesRunner, []string{}, &out, &errb)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("want ErrUsage, got %v", err)
	}
	if s := errb.String(); !bytes.Contains([]byte(s), []byte("load")) ||
		!bytes.Contains([]byte(s), []byte("distribute")) {
		t.Errorf("usage should list load + distribute: %s", s)
	}
}

func TestImagesLoadRequiredFlags(t *testing.T) {
	var out, errb bytes.Buffer
	// --jump alone is not enough; --kubeconfig, --env are also required; --inventory is NOT required
	err := loadCmd(context.Background(), nil, []string{"--jump", "jh"}, &out, &errb)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("want ErrUsage on missing flags, got %v", err)
	}
	// inventory must NOT appear in the required-flags error message
	if msg := errb.String(); strings.Contains(msg, "--inventory") {
		t.Errorf("--inventory should not be a required flag; got: %s", msg)
	}
}

var _ clusteraccess.Execer = (*imgExecer)(nil)

// imgInventoryYAML is a minimal valid Ansible inventory accepted by parseInventory.
// Uses recognized group names (worker/controlplane/lb) and includes ansible_user
// so Load yields >=1 node (the Task 1 non-empty-but-nodeless guard).
const imgInventoryYAML = `
all:
  children:
    worker:
      hosts:
        worker-1:
          ansible_host: 10.0.0.21
  vars:
    ansible_user: dune
    ansible_ssh_private_key_file: /home/dune/.ssh/id_ed25519
`
