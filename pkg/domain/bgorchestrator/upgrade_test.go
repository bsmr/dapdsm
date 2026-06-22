package bgorchestrator

import (
	"context"
	"strings"
	"testing"
	"time"
)

// scriptRunner serves a CR that flips to "ready" after the Start patch, so the
// final Ready gate resolves. It records the ordered patch types it received.
type scriptRunner struct {
	started bool
	patches []string // "merge:stop=true" | "merge:stop=false" | "json"
}

func (s *scriptRunner) Get(ctx context.Context, args ...string) ([]byte, error) {
	// Stopped while not started; Ready after Start.
	if s.started {
		return crReady(true), nil
	}
	return crReady(false), nil
}
func (s *scriptRunner) Patch(ctx context.Context, resource, name, ns, ptype, body string) error {
	switch {
	case ptype == "json":
		s.patches = append(s.patches, "json")
	case strings.Contains(body, "true"):
		s.patches = append(s.patches, "merge:stop=true")
	default:
		s.patches = append(s.patches, "merge:stop=false")
		s.started = true
	}
	return nil
}
func (s *scriptRunner) DeletePods(context.Context, string, ...string) error { return nil }
func (s *scriptRunner) Exec(context.Context, string, string, ...string) ([]byte, error) {
	return nil, nil
}
func (s *scriptRunner) ExecPiped(context.Context, string, string, []byte, ...string) ([]byte, error) {
	return nil, nil
}

func TestUpgradeRunsStopUpdateStartInOrder(t *testing.T) {
	r := &scriptRunner{}
	var phases []string
	cfg := Config{
		Poll:    time.Millisecond,
		Timeout: time.Second,
		OnPhase: func(p string) { phases = append(phases, p) },
	}
	err := Upgrade(context.Background(), r, "ns", "x", "9-0-shipping", cfg)
	if err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	want := []string{"merge:stop=true", "merge:stop=false"}
	// Update emits a json patch only if the CR has a placeholder tag; crReady
	// has none, so Update is a no-op here and we assert just stop then start.
	if len(r.patches) != len(want) {
		t.Fatalf("patch order = %v, want %v", r.patches, want)
	}
	for i := range want {
		if r.patches[i] != want[i] {
			t.Fatalf("patch[%d]=%q want %q (all: %v)", i, r.patches[i], want[i], r.patches)
		}
	}
	if len(phases) == 0 || phases[len(phases)-1] != "ready" {
		t.Fatalf("expected final phase 'ready', got %v", phases)
	}
}

func TestUpgradeAbortsWhenStopGateTimesOut(t *testing.T) {
	// A runner whose CR never drains (always ready) → Stopped gate times out.
	r := &alwaysReady{}
	cfg := Config{Poll: time.Millisecond, Timeout: 20 * time.Millisecond}
	if err := Upgrade(context.Background(), r, "ns", "x", "9-0-shipping", cfg); err == nil {
		t.Fatal("want stop-gate timeout error, got nil")
	}
}

type alwaysReady struct{ scriptRunner }

func (a *alwaysReady) Get(ctx context.Context, args ...string) ([]byte, error) {
	return crReady(true), nil
}
