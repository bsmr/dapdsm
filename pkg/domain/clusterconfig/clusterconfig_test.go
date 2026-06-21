package clusterconfig

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeKC struct {
	getOut  string
	getErr  error
	applied []string // captured manifests
}

func (f *fakeKC) Get(_ context.Context, _ ...string) ([]byte, error) {
	return []byte(f.getOut), f.getErr
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
		"namespace: dapdsm-system"} {
		if !strings.Contains(all, want) {
			t.Fatalf("manifest missing %q\n%s", want, all)
		}
	}
}

func TestLoad_ParsesValuesAndDecodesSecrets(t *testing.T) {
	// Save renders `kubectl get ... -o json`; emulate the merged JSON the Store reads.
	f := &fakeKC{getOut: `{"cm":{"WORLD_NAME":"Arrakis"},"secret":{"fls-token":"dG9r"}}`}
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
	f := &fakeKC{getErr: errors.New("Error from server (NotFound): configmaps \"x\" not found")}
	_, err := (Store{KC: f, Namespace: "dapdsm-system"}).Load(context.Background(), "x")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
