package imageload

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// fakeKubectl records calls and returns canned output keyed by the joined argv.
type fakeKubectl struct {
	out    map[string]string // argv-substring -> stdout
	runs   [][]string
	stdins [][]string // {joinedArgv, string(stdin)}
	err    error
}

func (f *fakeKubectl) Run(_ context.Context, args ...string) (string, error) {
	f.runs = append(f.runs, args)
	for k, v := range f.out {
		if strings.Contains(strings.Join(args, " "), k) {
			return v, f.err
		}
	}
	return "", f.err
}

func (f *fakeKubectl) Stdin(_ context.Context, stdin []byte, args ...string) (string, error) {
	f.stdins = append(f.stdins, []string{strings.Join(args, " "), string(stdin)})
	return "", f.err
}

func TestImporterPods(t *testing.T) {
	f := &fakeKubectl{out: map[string]string{
		"get pods": "pod-a pod-b pod-c",
	}}
	pods, err := importerPods(context.Background(), f, "ds-arrakis-imageload")
	if err != nil {
		t.Fatalf("importerPods: %v", err)
	}
	if want := []string{"pod-a", "pod-b", "pod-c"}; !reflect.DeepEqual(pods, want) {
		t.Errorf("pods = %v, want %v", pods, want)
	}
	// must query by label in the right namespace, jsonpath of names
	joined := strings.Join(f.runs[0], " ")
	for _, want := range []string{"get pods", "-n ds-arrakis-imageload", "-l app=ds-arrakis-imageload", "jsonpath={.items[*].metadata.name}"} {
		if !strings.Contains(joined, want) {
			t.Errorf("get-pods argv missing %q: %s", want, joined)
		}
	}
	// -o must be a discrete argv element, not fused into another token
	hasO := false
	for _, arg := range f.runs[0] {
		if arg == "-o" {
			hasO = true
			break
		}
	}
	if !hasO {
		t.Errorf("get-pods argv: -o must appear as a discrete element, got %v", f.runs[0])
	}
}

func TestImporterPodsEmpty(t *testing.T) {
	f := &fakeKubectl{out: map[string]string{"get pods": "  \n "}}
	if _, err := importerPods(context.Background(), f, "ns"); err == nil {
		t.Fatal("expected error when no importer pods are found")
	}
}

func TestImportTar(t *testing.T) {
	f := &fakeKubectl{}
	tar := []byte("TARBYTES\x00\x01\x02") // includes NULs — binary must survive
	err := importTar(context.Background(), f, "ds-arrakis-imageload", "pod-a",
		"/var/lib/rancher/rke2/bin/ctr", tar)
	if err != nil {
		t.Fatalf("importTar: %v", err)
	}
	if len(f.stdins) != 1 {
		t.Fatalf("want 1 Stdin call, got %d", len(f.stdins))
	}
	argv, stdin := f.stdins[0][0], f.stdins[0][1]
	for _, want := range []string{
		"exec -i pod-a", "-n ds-arrakis-imageload", "--",
		"/host/bin/ctr", "-a /host/containerd.sock", "-n k8s.io", "images import -",
	} {
		if !strings.Contains(argv, want) {
			t.Errorf("exec argv missing %q: %s", want, argv)
		}
	}
	if stdin != string(tar) {
		t.Errorf("tar bytes not passed verbatim as stdin")
	}
}

// fakeReader returns canned bytes per path.
type fakeReader struct{ files map[string][]byte }

func (f fakeReader) ReadFile(_ context.Context, p string) ([]byte, error) {
	b, ok := f.files[p]
	if !ok {
		return nil, fmt.Errorf("no such file %s", p)
	}
	return b, nil
}

func TestLoadFlow(t *testing.T) {
	f := &fakeKubectl{out: map[string]string{"get pods": "pod-a pod-b"}}
	r := fakeReader{files: map[string][]byte{
		"/d/op1.tar": []byte("one"),
		"/d/op2.tar": []byte("two"),
	}}
	opts := Options{
		Namespace: "ds-arrakis-imageload",
		Tars:      []string{"/d/op1.tar", "/d/op2.tar"},
		Socket:    "/run/k3s/containerd/containerd.sock",
		CtrPath:   "/var/lib/rancher/rke2/bin/ctr",
	}
	res, err := Load(context.Background(), f, r, opts)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// 2 tars x 2 pods = 4 import streams
	if len(f.stdins) != 5 { // 1 apply + 4 imports
		t.Errorf("want 5 Stdin calls (1 apply + 4 imports), got %d", len(f.stdins))
	}
	if len(res.Pods) != 2 || len(res.Tars) != 2 {
		t.Errorf("result = %+v", res)
	}
	joined := func() string {
		var b strings.Builder
		for _, r := range f.runs {
			b.WriteString(strings.Join(r, " ") + "\n")
		}
		return b.String()
	}()
	if !strings.Contains(joined, "rollout status") {
		t.Errorf("missing rollout wait:\n%s", joined)
	}
	if !strings.Contains(joined, "delete namespace ds-arrakis-imageload") {
		t.Errorf("missing teardown:\n%s", joined)
	}
}

func TestLoadKeepDaemonSkipsTeardown(t *testing.T) {
	f := &fakeKubectl{out: map[string]string{"get pods": "pod-a"}}
	r := fakeReader{files: map[string][]byte{"/d/op1.tar": []byte("x")}}
	_, err := Load(context.Background(), f, r, Options{
		Namespace: "ns", Tars: []string{"/d/op1.tar"},
		Socket: "/s", CtrPath: "/b/ctr", KeepDaemon: true,
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, run := range f.runs {
		if strings.Contains(strings.Join(run, " "), "delete namespace") {
			t.Error("teardown ran despite KeepDaemon")
		}
	}
}

func TestLoadGuards(t *testing.T) {
	r := fakeReader{}
	// empty tars must be rejected
	if _, err := Load(context.Background(), &fakeKubectl{}, r, Options{}); err == nil {
		t.Error("expected error on empty tars")
	}
}
