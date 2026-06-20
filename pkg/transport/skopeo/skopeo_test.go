package skopeo

import (
	"context"
	"strings"
	"testing"
)

type fakeRunner struct {
	calls  [][]string
	tarOut string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	if name == "tar" {
		return f.tarOut, nil
	}
	return "", nil
}

func TestEnsureInstalled_IdempotentSudoBash(t *testing.T) {
	f := &fakeRunner{}
	if err := EnsureInstalled(context.Background(), f); err != nil {
		t.Fatalf("EnsureInstalled: %v", err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(f.calls))
	}
	c := f.calls[0]
	if c[0] != "sudo" || c[1] != "bash" || c[2] != "-c" {
		t.Fatalf("call = %v, want [sudo bash -c <script>]", c)
	}
	for _, want := range []string{"command -v skopeo", "apt-get install -y skopeo"} {
		if !strings.Contains(c[3], want) {
			t.Errorf("install script missing %q", want)
		}
	}
}

func TestRepoTags_ParsesManifest(t *testing.T) {
	// docker-archive manifest.json shape.
	f := &fakeRunner{tarOut: `[{"Config":"c.json","RepoTags":["funcom/dune-operator:v1.5.0"],"Layers":["l1"]}]`}
	tags, err := RepoTags(context.Background(), f, "/home/dune/depot/prod/images/operators/op.tar")
	if err != nil {
		t.Fatalf("RepoTags: %v", err)
	}
	if len(tags) != 1 || tags[0] != "funcom/dune-operator:v1.5.0" {
		t.Fatalf("tags = %v", tags)
	}
	// must read manifest.json out of the named tar
	c := f.calls[0]
	if c[0] != "tar" || c[len(c)-1] != "manifest.json" {
		t.Errorf("tar call = %v", c)
	}
	if c[len(c)-2] != "/home/dune/depot/prod/images/operators/op.tar" {
		t.Errorf("tar path not passed: %v", c)
	}
}

func TestCopy_BuildsInsecureCopyArgs(t *testing.T) {
	f := &fakeRunner{}
	err := Copy(context.Background(), f,
		"docker-archive:/d/op.tar:funcom/dune-operator:v1.5.0",
		"docker://reg:5000/funcom/dune-operator:v1.5.0")
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}
	joined := strings.Join(f.calls[0], " ")
	for _, want := range []string{
		"skopeo copy",
		"--dest-tls-verify=false",
		"docker-archive:/d/op.tar:funcom/dune-operator:v1.5.0",
		"docker://reg:5000/funcom/dune-operator:v1.5.0",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("copy args missing %q: %s", want, joined)
		}
	}
}
