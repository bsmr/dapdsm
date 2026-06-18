package bootstrap

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.muehmer.eu/dapdsm/pkg/transport/kubedist"
)

type stubResolver struct{ ip string }

func (s stubResolver) Resolve(context.Context) (string, error) { return s.ip, nil }

type stubSteam struct{ updated uint32 }

func (s *stubSteam) EnsureInstalled(context.Context) error                       { return nil }
func (s *stubSteam) AppUpdate(_ context.Context, appID uint32, _ string) error   { s.updated = appID; return nil }

type stubApplier struct{ args []string }

func (s *stubApplier) Apply(_ context.Context, args ...string) error { s.args = args; return nil }

type stubDistro struct {
	installed, ready, imported bool
	gotCfg                     kubedist.Config
}

func (s *stubDistro) Name() string                                   { return "k3s" }
func (s *stubDistro) Install(_ context.Context, c kubedist.Config) error { s.installed = true; s.gotCfg = c; return nil }
func (s *stubDistro) EnsureReady(context.Context) error              { s.ready = true; return nil }
func (s *stubDistro) ImportImages(context.Context, string) error     { s.imported = true; return nil }
func (s *stubDistro) Kubeconfig() string                             { return "/k.yaml" }
func (s *stubDistro) Uninstall(context.Context) error                { return nil }

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestAppID(t *testing.T) {
	if id, _ := AppID("prod"); id != 4754530 {
		t.Fatalf("prod=%d", id)
	}
	if id, _ := AppID("test"); id != 3104830 {
		t.Fatalf("test=%d", id)
	}
	if _, err := AppID("staging"); err == nil {
		t.Fatal("want error")
	}
}

func TestRun_FullSequence(t *testing.T) {
	d := &stubDistro{}
	steam := &stubSteam{}
	ap := &stubApplier{}
	// operators dir must exist for the locate step:
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "images", "operators", "crds"))

	err := Run(context.Background(), Config{Target: "test", Distro: "k3s", DownloadDir: dir},
		Deps{Distro: d, Resolver: stubResolver{ip: "1.2.3.4"}, Steam: steam, Applier: ap,
			Stdout: io.Discard, ReadyTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if !d.installed || !d.ready || !d.imported {
		t.Fatalf("distro steps: %+v", d)
	}
	if steam.updated != 3104830 {
		t.Fatalf("app id: %d", steam.updated)
	}
	wantCRDPath := filepath.Join(dir, "images", "operators", "crds")
	if len(ap.args) != 2 || ap.args[0] != "-f" || ap.args[1] != wantCRDPath {
		t.Fatalf("apply args: %v", ap.args)
	}
	if d.gotCfg.ExternalIP != "1.2.3.4" {
		t.Fatalf("resolved external IP did not reach Install: %q", d.gotCfg.ExternalIP)
	}
}

func TestRun_ErrorsWhenOperatorsMissing(t *testing.T) {
	err := Run(context.Background(), Config{Target: "prod", Distro: "k3s", DownloadDir: t.TempDir()},
		Deps{Distro: &stubDistro{}, Resolver: stubResolver{ip: "9.9.9.9"}, Steam: &stubSteam{},
			Applier: &stubApplier{}, Stdout: io.Discard, ReadyTimeout: time.Second})
	if !errors.Is(err, ErrNoOperators) {
		t.Fatalf("want ErrNoOperators, got %v", err)
	}
}
