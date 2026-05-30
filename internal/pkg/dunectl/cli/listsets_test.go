package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type listSetsRunner struct{}

func (listSetsRunner) Get(_ context.Context, args ...string) ([]byte, error) {
	if len(args) >= 1 && args[0] == "ns" {
		return []byte("funcom-seabass-sh-deadbeef\n"), nil
	}
	return []byte(setScalingCR), nil
}
func (listSetsRunner) Patch(context.Context, string, string, string, string, string) error {
	return nil
}
func (listSetsRunner) DeletePods(context.Context, string, ...string) error { return nil }
func (listSetsRunner) Exec(context.Context, string, string, ...string) ([]byte, error) {
	return nil, nil
}
func (listSetsRunner) ExecPiped(context.Context, string, string, []byte, ...string) ([]byte, error) {
	return nil, nil
}

func TestListSets_Table(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	err := listSets(context.Background(), nil, &stdout, &stderr, listSetsDeps{runner: listSetsRunner{}})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	out := stdout.String()
	for _, want := range []string{
		"MAP",
		"Survival_1",
		"always-on",
		"SH_Arrakeen",
		"on-demand",
		"DeepDesert_1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\nstdout=%s", want, out)
		}
	}
}

func TestListSets_JSON(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	err := listSets(context.Background(), []string{"--json"}, &stdout, &stderr, listSetsDeps{runner: listSetsRunner{}})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, stdout.String())
	}
	if len(got) != 4 {
		t.Errorf("len = %d, want 4", len(got))
	}
}
