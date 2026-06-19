package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const portsCR = `{
  "spec": {"serverGroup": {"template": {"spec": {"sets": [
    {"map": "Survival_1", "arguments": ["-FarmRegion=Europe"]},
    {"map": "Overmap",    "arguments": ["-FarmRegion=Europe"]}
  ]}}}}
}`

type portsRunner struct {
	patchPayload string
	patchCalls   int
}

func (r *portsRunner) Get(_ context.Context, args ...string) ([]byte, error) {
	if len(args) >= 1 && args[0] == "ns" {
		return []byte("funcom-seabass-sh-deadbeef\n"), nil
	}
	if len(args) >= 1 && args[0] == "battlegroup" {
		return []byte(portsCR), nil
	}
	return nil, errors.New("unexpected Get " + strings.Join(args, " "))
}

func (r *portsRunner) Patch(_ context.Context, _, _, _, _, payload string) error {
	r.patchPayload = payload
	r.patchCalls++
	return nil
}

func (r *portsRunner) DeletePods(context.Context, string, ...string) error { return nil }
func (r *portsRunner) Exec(context.Context, string, string, ...string) ([]byte, error) {
	return nil, nil
}
func (r *portsRunner) ExecPiped(context.Context, string, string, []byte, ...string) ([]byte, error) {
	return nil, nil
}

func TestPatchGamePorts_PatchesEverySetWithMissingArgs(t *testing.T) {
	t.Parallel()
	r := &portsRunner{}
	var stdout, stderr bytes.Buffer
	err := runPatchGamePorts(context.Background(),
		[]string{"--game-base", "7877", "--igw-base", "7988"},
		&stdout, &stderr, patchGamePortsDeps{runner: r})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if r.patchCalls != 1 {
		t.Fatalf("patchCalls = %d, want 1", r.patchCalls)
	}
	var ops []map[string]any
	if err := json.Unmarshal([]byte(r.patchPayload), &ops); err != nil {
		t.Fatalf("payload not JSON: %v", err)
	}
	// 2 sets × 2 args = 4 add ops expected.
	if len(ops) != 4 {
		t.Errorf("len(ops) = %d, want 4\n  payload: %s", len(ops), r.patchPayload)
	}
}

func TestPatchGamePorts_NoOpWhenEverythingAlreadyCorrect(t *testing.T) {
	t.Parallel()
	// Both sets already carry the target Port and IGWPort → no patch call.
	r := &portsRunner{}
	// Override Get response for this test only — both sets carry target args.
	r.patchPayload = "" // reset
	already := `{
  "spec": {"serverGroup": {"template": {"spec": {"sets": [
    {"map": "Survival_1", "arguments": ["-ini:engine:[URL]:Port=7877", "-ini:engine:[URL]:IGWPort=7988"]},
    {"map": "Overmap",    "arguments": ["-ini:engine:[URL]:Port=7877", "-ini:engine:[URL]:IGWPort=7988"]}
  ]}}}}
}`
	deps := patchGamePortsDeps{runner: &alreadyRunner{cr: already}}
	var stdout, stderr bytes.Buffer
	err := runPatchGamePorts(context.Background(),
		[]string{"--game-base", "7877", "--igw-base", "7988"},
		&stdout, &stderr, deps)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stdout.String(), "no changes needed") {
		t.Errorf("stdout = %q, want 'no changes needed'", stdout.String())
	}
}

func TestPatchGamePorts_RequiresBothFlags(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	err := runPatchGamePorts(context.Background(), []string{"--game-base", "7877"},
		&stdout, &stderr, patchGamePortsDeps{runner: &portsRunner{}})
	if err == nil || !strings.Contains(err.Error(), "igw-base") {
		t.Errorf("err = %v, want missing-igw-base error", err)
	}
}

// alreadyRunner serves a configurable CR and rejects unexpected patches.
type alreadyRunner struct{ cr string }

func (a *alreadyRunner) Get(_ context.Context, args ...string) ([]byte, error) {
	if len(args) >= 1 && args[0] == "ns" {
		return []byte("funcom-seabass-sh-deadbeef\n"), nil
	}
	return []byte(a.cr), nil
}
func (a *alreadyRunner) Patch(context.Context, string, string, string, string, string) error {
	return errors.New("Patch must not be called when no ops are needed")
}
func (a *alreadyRunner) DeletePods(context.Context, string, ...string) error { return nil }
func (a *alreadyRunner) Exec(context.Context, string, string, ...string) ([]byte, error) {
	return nil, nil
}
func (a *alreadyRunner) ExecPiped(context.Context, string, string, []byte, ...string) ([]byte, error) {
	return nil, nil
}
