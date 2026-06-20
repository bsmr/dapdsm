package imagedist

import (
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/domain/depot"
)

// fakeRunner answers ls/tar and records skopeo copy calls.
type fakeRunner struct {
	calls   [][]string
	lsOut   map[string]string // dir -> newline-separated tar paths
	tagsOut map[string]string // tar path -> manifest.json
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	switch name {
	case "sh": // ls -1 <dir>/*.tar  (args: -c, "ls -1 <dir>/*.tar ...")
		for dir, out := range f.lsOut {
			if strings.Contains(args[len(args)-1], dir) {
				return out, nil
			}
		}
		return "", nil
	case "tar":
		return f.tagsOut[args[len(args)-2]], nil // path is second-to-last (… <path> manifest.json)
	}
	return "", nil
}

func TestPush_CopiesEveryImageIntoRegistry(t *testing.T) {
	f := &fakeRunner{
		lsOut: map[string]string{
			"/d/images/operators":     "/d/images/operators/op.tar\n",
			"/d/images/prerequisites": "/d/images/prerequisites/pre.tar\n",
			"/d/images/battlegroup":   "/d/images/battlegroup/bg.tar\n",
		},
		tagsOut: map[string]string{
			"/d/images/operators/op.tar":      `[{"RepoTags":["funcom/op:v1.5.0"]}]`,
			"/d/images/prerequisites/pre.tar": `[{"RepoTags":["funcom/pre:v1.5.0"]}]`,
			"/d/images/battlegroup/bg.tar":    `[{"RepoTags":["funcom/bg:v1.5.0"]}]`,
		},
	}
	d := depot.Result{
		OperatorsDir:     "/d/images/operators",
		PrerequisitesDir: "/d/images/prerequisites",
		BattlegroupDir:   "/d/images/battlegroup",
	}
	res, err := Push(context.Background(), f, "reg:5000", d)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if res.Registry != "reg:5000" {
		t.Errorf("Registry = %q", res.Registry)
	}
	if len(res.Images) != 3 {
		t.Fatalf("want 3 images, got %d: %+v", len(res.Images), res.Images)
	}
	// every image must have been skopeo-copied into the registry
	joined := ""
	for _, c := range f.calls {
		joined += strings.Join(c, " ") + "\n"
	}
	for _, want := range []string{
		"skopeo copy --dest-tls-verify=false docker-archive:/d/images/operators/op.tar:funcom/op:v1.5.0 docker://reg:5000/funcom/op:v1.5.0",
		"docker://reg:5000/funcom/pre:v1.5.0",
		"docker://reg:5000/funcom/bg:v1.5.0",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing copy %q in:\n%s", want, joined)
		}
	}
}

func TestPush_SplitsRepoAndTag(t *testing.T) {
	got := splitRef("funcom/op:v1.5.0")
	if got.Repo != "funcom/op" || got.Tag != "v1.5.0" {
		t.Errorf("splitRef = %+v", got)
	}
}
