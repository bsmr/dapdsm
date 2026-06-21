package clusterconfig

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

type fakeKC struct {
	cmOut, secOut string // `kubectl get -o json` output per resource kind
	cmErr, secErr error
	applied       []string // captured manifests
}

func (f *fakeKC) Get(_ context.Context, args ...string) ([]byte, error) {
	if slices.Contains(args, "secret") {
		return []byte(f.secOut), f.secErr
	}
	return []byte(f.cmOut), f.cmErr
}
func (f *fakeKC) Apply(_ context.Context, manifest []byte) error {
	f.applied = append(f.applied, string(manifest))
	return nil
}

func TestSave_RendersConfigMapAndSecret(t *testing.T) {
	f := &fakeKC{}
	s := Store{KC: f, Namespace: "dapdsm-system"}
	err := s.Save(context.Background(), "dapdsm-bg-config", Data{
		Values:  map[string]string{"WORLD_NAME": "Arrakis"},
		Secrets: map[string][]byte{"fls-token": []byte("tok")},
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	all := strings.Join(f.applied, "\n---\n")
	for _, want := range []string{"kind: ConfigMap", "name: dapdsm-bg-config", "WORLD_NAME: Arrakis",
		"kind: Secret", "name: dapdsm-bg-config-secrets", "stringData:", "fls-token: tok",
		"namespace: dapdsm-system", "kind: Namespace"} {
		if !strings.Contains(all, want) {
			t.Fatalf("manifest missing %q\n%s", want, all)
		}
	}
	// The namespace must be applied before the ConfigMap/Secret land in it.
	if !strings.Contains(f.applied[0], "kind: Namespace") {
		t.Fatalf("namespace not applied first; got %q", f.applied[0])
	}
}

func TestLoad_ParsesValuesAndDecodesSecrets(t *testing.T) {
	// Realistic `kubectl get configmap|secret -o json` documents (Load reads .data).
	f := &fakeKC{
		cmOut:  `{"apiVersion":"v1","kind":"ConfigMap","data":{"WORLD_NAME":"Arrakis"}}`,
		secOut: `{"apiVersion":"v1","kind":"Secret","data":{"fls-token":"dG9r"}}`,
	}
	s := Store{KC: f, Namespace: "dapdsm-system"}
	d, err := s.Load(context.Background(), "dapdsm-bg-config")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d.Values["WORLD_NAME"] != "Arrakis" {
		t.Fatalf("Values = %v", d.Values)
	}
	if string(d.Secrets["fls-token"]) != "tok" { // dG9r == base64("tok")
		t.Fatalf("Secrets = %v", d.Secrets)
	}
}

func TestLoad_NotFound(t *testing.T) {
	// The ConfigMap (existence signal) is absent → ErrNotFound. The adapter
	// surfaces kubectl's stderr into the error (see ccKubectl.Get).
	f := &fakeKC{cmErr: errors.New(`exit status 1: Error from server (NotFound): configmaps "x" not found`)}
	_, err := (Store{KC: f, Namespace: "dapdsm-system"}).Load(context.Background(), "x")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestLoad_ConfigMapPresentSecretMissing(t *testing.T) {
	f := &fakeKC{
		cmOut:  `{"data":{"WORLD_NAME":"Arrakis"}}`,
		secErr: errors.New(`exit status 1: Error from server (NotFound): secrets "x-secrets" not found`),
	}
	d, err := (Store{KC: f, Namespace: "dapdsm-system"}).Load(context.Background(), "x")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d.Values["WORLD_NAME"] != "Arrakis" || len(d.Secrets) != 0 {
		t.Fatalf("want values only, got Values=%v Secrets=%v", d.Values, d.Secrets)
	}
}
