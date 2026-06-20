package imagedist

import (
	"context"
	"strings"
	"testing"
)

type fakeKubectl struct {
	applied  []byte
	runCalls [][]string
}

func (f *fakeKubectl) Apply(_ context.Context, manifest []byte) (string, error) {
	f.applied = manifest
	return "", nil
}
func (f *fakeKubectl) Run(_ context.Context, args ...string) (string, error) {
	f.runCalls = append(f.runCalls, args)
	return "", nil
}

func TestDeploy_AppliesManifestThenWaits(t *testing.T) {
	f := &fakeKubectl{}
	opts := Options{
		Namespace: "funcom-registry", StorageClass: "local-path",
		RegistryImage: "registry:2.8.3", Endpoint: "10.0.0.9:5000",
		PVCSize: "10Gi", LoadBalancerIP: "10.0.0.9",
	}
	if err := Deploy(context.Background(), f, opts); err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	m := string(f.applied)
	for _, want := range []string{
		"kind: Namespace", "name: funcom-registry",
		"storageClassName: local-path", "storage: 10Gi",
		"image: registry:2.8.3", "type: LoadBalancer",
		"loadBalancerIP: 10.0.0.9", "port: 5000",
	} {
		if !strings.Contains(m, want) {
			t.Errorf("manifest missing %q", want)
		}
	}
	// must wait for the registry to be ready after applying
	sawWait := false
	for _, c := range f.runCalls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "rollout status") && strings.Contains(j, "registry") {
			sawWait = true
		}
	}
	if !sawWait {
		t.Errorf("Deploy did not wait for rollout: %v", f.runCalls)
	}
}

func TestRender_OmitsLoadBalancerIPWhenEmpty(t *testing.T) {
	out, err := render(Options{
		Namespace: "ns", StorageClass: "sc", RegistryImage: "registry:2.8.3",
		Endpoint: "host:5000", PVCSize: "10Gi",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(string(out), "loadBalancerIP") {
		t.Errorf("loadBalancerIP should be omitted when empty:\n%s", out)
	}
}
