package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const setScalingCR = `{
  "spec": {
    "serverGroup": {
      "template": {
        "spec": {
          "sets": [
            {"map": "Survival_1",      "dedicatedScaling": false, "replicas": 1, "partitions": [1]},
            {"map": "SH_Arrakeen",     "dedicatedScaling": true},
            {"map": "SH_HarkoVillage", "dedicatedScaling": true},
            {"map": "DeepDesert_1",    "dedicatedScaling": true}
          ]
        }
      }
    }
  }
}`

const setScalingDBDeployment = `{
  "items": [
    {
      "metadata": {"name": "sh-deadbeef-db-dbdepl"},
      "spec": {"port": 15432, "superUser": "postgres", "superPassword": "x", "gameDatabaseName": "dune"}
    }
  ]
}`

// world_partition rows for every map in setScalingCR. Mirrors the
// observed real-cluster mapping (partition_id matches the set index + 1).
const setScalingWorldPartitionRows = "Survival_1|1\nSH_Arrakeen|2\nSH_HarkoVillage|3\nDeepDesert_1|4\n"

// setScalingRunner serves the canned BattleGroup CR + DatabaseDeployment +
// psql world_partition response, and records patches.
type setScalingRunner struct {
	patchPayload string
	patchCalls   int
}

func (r *setScalingRunner) Get(_ context.Context, args ...string) ([]byte, error) {
	if len(args) >= 1 && args[0] == "ns" {
		return []byte("funcom-seabass-sh-deadbeef\n"), nil
	}
	if len(args) >= 1 && args[0] == "battlegroup" {
		return []byte(setScalingCR), nil
	}
	if len(args) >= 1 && args[0] == "databasedeployment" {
		return []byte(setScalingDBDeployment), nil
	}
	return nil, errors.New("unexpected Get")
}

func (r *setScalingRunner) Patch(_ context.Context, _, _, _, _, payload string) error {
	r.patchPayload = payload
	r.patchCalls++
	return nil
}

func (r *setScalingRunner) DeletePods(context.Context, string, ...string) error { return nil }

func (r *setScalingRunner) Exec(_ context.Context, _, _ string, command ...string) ([]byte, error) {
	if len(command) > 0 && strings.Contains(strings.Join(command, " "), "world_partition") {
		return []byte(setScalingWorldPartitionRows), nil
	}
	return nil, nil
}

func (r *setScalingRunner) ExecPiped(context.Context, string, string, []byte, ...string) ([]byte, error) {
	return nil, nil
}

func TestEnableSet_PatchesRequestedMaps(t *testing.T) {
	t.Parallel()
	r := &setScalingRunner{}
	var stdout, stderr bytes.Buffer
	err := runSetScaling(context.Background(), "enable-set",
		[]string{"SH_Arrakeen", "DeepDesert_1"},
		&stdout, &stderr, setScalingDeps{runner: r},
		false, 1, true)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if r.patchCalls != 1 {
		t.Fatalf("patchCalls = %d, want 1", r.patchCalls)
	}

	var ops []map[string]any
	if err := json.Unmarshal([]byte(r.patchPayload), &ops); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	// SH_Arrakeen has neither replicas nor partitions → "replace" scaling
	// + "add" replicas + "add" partitions. DeepDesert_1 same. So 6 ops total.
	if len(ops) != 6 {
		t.Errorf("len(ops) = %d, want 6 (2 maps × {scaling, replicas, partitions})", len(ops))
	}
	for _, op := range ops {
		if op["op"] == "replace" && op["path"] == "/spec/serverGroup/template/spec/sets/1/dedicatedScaling" && op["value"] != false {
			t.Errorf("scaling op for SH_Arrakeen value = %v, want false", op["value"])
		}
	}
}

func TestEnableSet_ReplicasFlagOverridesDefault(t *testing.T) {
	t.Parallel()
	r := &setScalingRunner{}
	var stdout, stderr bytes.Buffer
	err := runSetScaling(context.Background(), "enable-set",
		[]string{"--replicas", "3", "SH_Arrakeen"},
		&stdout, &stderr, setScalingDeps{runner: r},
		false, 1, true)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(r.patchPayload, `"value":3`) {
		t.Errorf("payload = %s, want a replicas=3 op", r.patchPayload)
	}
	if !strings.Contains(stdout.String(), "replicas=3") {
		t.Errorf("stdout = %q, want replicas=3 in summary", stdout.String())
	}
}

func TestDisableSet_PatchesTargetState(t *testing.T) {
	t.Parallel()
	r := &setScalingRunner{}
	var stdout, stderr bytes.Buffer
	err := runSetScaling(context.Background(), "disable-set",
		[]string{"Survival_1"},
		&stdout, &stderr, setScalingDeps{runner: r},
		true, 0, false)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if r.patchCalls != 1 {
		t.Fatalf("patchCalls = %d, want 1", r.patchCalls)
	}
	if !strings.Contains(r.patchPayload, `"value":true`) {
		t.Errorf("payload = %s, want dedicatedScaling=true op", r.patchPayload)
	}
	if !strings.Contains(r.patchPayload, `"value":0`) {
		t.Errorf("payload = %s, want replicas=0 op", r.patchPayload)
	}
}

func TestSetScaling_UnknownMapIsAFatalError(t *testing.T) {
	t.Parallel()
	r := &setScalingRunner{}
	var stdout, stderr bytes.Buffer
	err := runSetScaling(context.Background(), "enable-set",
		[]string{"DoesNotExist"},
		&stdout, &stderr, setScalingDeps{runner: r},
		false, 1, true)
	if err == nil || !strings.Contains(err.Error(), "DoesNotExist") {
		t.Fatalf("err = %v, want error mentioning DoesNotExist", err)
	}
	if r.patchCalls != 0 {
		t.Errorf("patchCalls = %d, want 0 (no patch when any map missing)", r.patchCalls)
	}
}

func TestSetScaling_NoMapsReturnsErrUsage(t *testing.T) {
	t.Parallel()
	r := &setScalingRunner{}
	var stdout, stderr bytes.Buffer
	err := runSetScaling(context.Background(), "enable-set",
		[]string{}, &stdout, &stderr, setScalingDeps{runner: r},
		false, 1, true)
	if !errors.Is(err, ErrUsage) {
		t.Errorf("err = %v, want errors.Is(err, ErrUsage)", err)
	}
}

func TestSetScaling_AlreadyInTargetStateIsNoOp(t *testing.T) {
	t.Parallel()
	r := &setScalingRunner{}
	var stdout, stderr bytes.Buffer
	err := runSetScaling(context.Background(), "enable-set",
		[]string{"Survival_1"}, // already always-on, replicas=1
		&stdout, &stderr, setScalingDeps{runner: r},
		false, 1, true)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if r.patchCalls != 0 {
		t.Errorf("patchCalls = %d, want 0 (no-op)", r.patchCalls)
	}
	if !strings.Contains(stdout.String(), "no changes needed") {
		t.Errorf("stdout = %q, want 'no changes needed'", stdout.String())
	}
}
