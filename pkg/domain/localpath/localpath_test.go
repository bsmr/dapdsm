package localpath

import (
	"context"
	"strings"
	"testing"
)

type fakeKC struct {
	applied  [][]byte
	runCalls [][]string
}

func (f *fakeKC) Run(_ context.Context, args ...string) (string, error) {
	f.runCalls = append(f.runCalls, args)
	if len(args) >= 2 && args[0] == "get" && args[1] == "storageclass" {
		return "local-path", nil
	}
	return "", nil
}
func (f *fakeKC) Apply(_ context.Context, m []byte) (string, error) {
	f.applied = append(f.applied, m)
	return "", nil
}

func TestInstall_LoadsAppliesVerifies(t *testing.T) {
	kc := &fakeKC{}
	loaded := false
	err := Install(context.Background(), kc, func(context.Context) error { loaded = true; return nil },
		Options{DepotDir: "/home/dune/depot/prod"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !loaded {
		t.Error("provisioner image was not loaded")
	}
	if len(kc.applied) != 1 || !strings.Contains(string(kc.applied[0]), "local-path") {
		t.Errorf("local-path manifest not applied: %v", kc.applied)
	}
	// verifies the StorageClass exists.
	var verified bool
	for _, c := range kc.runCalls {
		if len(c) >= 2 && c[0] == "get" && c[1] == "storageclass" {
			verified = true
		}
	}
	if !verified {
		t.Error("Install did not verify the StorageClass")
	}
}
