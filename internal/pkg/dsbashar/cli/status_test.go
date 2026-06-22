package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

type statusRunner struct{}

func (statusRunner) Get(_ context.Context, args ...string) ([]byte, error) {
	if len(args) >= 1 && args[0] == "ns" {
		return []byte("funcom-seabass-sh-deadbeef\n"), nil
	}
	return []byte(`{"status":{
		"phase":"Running","serverGroupPhase":"Running","size":2,
		"startTimestamp":"2026-06-21T10:00:00Z",
		"database":{"phase":"Ready"},"utilities":{"director":{"phase":"Ready"}},
		"servers":[
		  {"partitionMap":"Survival_1","phase":"Running","ready":true,"gamePort":7777,"igwPort":7888},
		  {"partitionMap":"Overmap","phase":"Stopped","ready":false,"restarts":2,"exitReason":"SIGSEGV","exitCode":139}
		]}}`), nil
}
func (statusRunner) Patch(context.Context, string, string, string, string, string) error { return nil }
func (statusRunner) DeletePods(context.Context, string, ...string) error                 { return nil }
func (statusRunner) Exec(context.Context, string, string, ...string) ([]byte, error)     { return nil, nil }
func (statusRunner) ExecPiped(context.Context, string, string, []byte, ...string) ([]byte, error) {
	return nil, nil
}

func TestStatus_Table(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if err := runStatus(context.Background(), nil, &stdout, &stderr, statusDeps{runner: statusRunner{}}); err != nil {
		t.Fatalf("err = %v", err)
	}
	out := stdout.String()
	for _, want := range []string{
		"battlegroup: sh-deadbeef", "phase:", "Running", "serverGroup:",
		"database:", "Ready", "MAP", "Survival_1", "Overmap", "7777", "SIGSEGV(139)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\nstdout=%s", want, out)
		}
	}
}

func TestStatus_JSON(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if err := runStatus(context.Background(), []string{"--json"}, &stdout, &stderr, statusDeps{runner: statusRunner{}}); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stdout.String(), `"phase": "Running"`) {
		t.Errorf("json missing phase=Running\n%s", stdout.String())
	}
}
