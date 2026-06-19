package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dsbashar/config"
)

type recordedRun struct {
	stdin []byte
	name  string
	args  []string
}

// fakeSetupRun captures every invocation of the run-func; returns ok by default.
type fakeSetupRun struct {
	calls []recordedRun
	err   error
}

func (f *fakeSetupRun) Run(_ context.Context, stdin []byte, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, recordedRun{stdin: stdin, name: name, args: args})
	return nil, f.err
}

func TestRunSetup_PipesNameRegionTokenInOrder(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		Target:       config.TargetProd,
		FLSTokenFile: "/etc/dune/fls-token-prod",
		WorldName:    "HADESNET",
		WorldRegion:  "Europe",
	}
	run := &fakeSetupRun{}
	deps := setupDeps{
		cfg:      cfg,
		run:      run.Run,
		tokenSrc: func() ([]byte, error) { return []byte("eyJ.TOKEN.STRING"), nil },
	}
	var stdout, stderr bytes.Buffer
	if err := runSetup(context.Background(), nil, &stdout, &stderr, deps); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(run.calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(run.calls))
	}
	want := "HADESNET\n2\neyJ.TOKEN.STRING\n"
	if string(run.calls[0].stdin) != want {
		t.Errorf("stdin = %q, want %q", run.calls[0].stdin, want)
	}
}

func TestRunSetup_RejectsUnsafeWorldName(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		Target:      config.TargetProd,
		WorldName:   "Hadesnet: Offworld",
		WorldRegion: "Europe",
	}
	run := &fakeSetupRun{}
	deps := setupDeps{cfg: cfg, run: run.Run, tokenSrc: func() ([]byte, error) { return []byte("x"), nil }}
	var stdout, stderr bytes.Buffer
	err := runSetup(context.Background(), nil, &stdout, &stderr, deps)
	if err == nil || !strings.Contains(err.Error(), "WORLD_NAME") {
		t.Errorf("err = %v, want WORLD_NAME validation error", err)
	}
	if len(run.calls) != 0 {
		t.Errorf("setup.sh was invoked despite invalid WORLD_NAME")
	}
}

func TestRunSetup_RejectsUnknownRegion(t *testing.T) {
	t.Parallel()
	cfg := config.Config{Target: config.TargetProd, WorldName: "OK", WorldRegion: "Atlantis"}
	run := &fakeSetupRun{}
	deps := setupDeps{cfg: cfg, run: run.Run, tokenSrc: func() ([]byte, error) { return []byte("x"), nil }}
	var stdout, stderr bytes.Buffer
	err := runSetup(context.Background(), nil, &stdout, &stderr, deps)
	if err == nil || !strings.Contains(err.Error(), "Atlantis") {
		t.Errorf("err = %v, want region error mentioning Atlantis", err)
	}
}

func TestRunSetup_PropagatesTokenSourceError(t *testing.T) {
	t.Parallel()
	cfg := config.Config{Target: config.TargetProd, WorldName: "OK", WorldRegion: "Europe"}
	deps := setupDeps{
		cfg:      cfg,
		run:      (&fakeSetupRun{}).Run,
		tokenSrc: func() ([]byte, error) { return nil, errors.New("permission denied") },
	}
	var stdout, stderr bytes.Buffer
	err := runSetup(context.Background(), nil, &stdout, &stderr, deps)
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("err = %v, want propagated token error", err)
	}
}

func TestRunSetup_RequiresWorldName(t *testing.T) {
	t.Parallel()
	cfg := config.Config{Target: config.TargetProd, WorldRegion: "Europe"}
	deps := setupDeps{cfg: cfg, run: (&fakeSetupRun{}).Run, tokenSrc: func() ([]byte, error) { return []byte("x"), nil }}
	var stdout, stderr bytes.Buffer
	err := runSetup(context.Background(), nil, &stdout, &stderr, deps)
	if err == nil || !strings.Contains(err.Error(), "WORLD_NAME") {
		t.Errorf("err = %v, want missing-WORLD_NAME error", err)
	}
}
