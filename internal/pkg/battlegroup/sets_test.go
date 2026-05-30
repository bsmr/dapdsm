package battlegroup

import (
	"slices"
	"testing"
)

const setsSampleCR = `{
  "spec": {
    "serverGroup": {
      "template": {
        "spec": {
          "sets": [
            {"map": "Survival_1",      "dedicatedScaling": false, "replicas": 1, "partitions": [1]},
            {"map": "Overmap",         "dedicatedScaling": false, "replicas": 1},
            {"map": "SH_Arrakeen",     "dedicatedScaling": true},
            {"map": "SH_HarkoVillage", "dedicatedScaling": true, "replicas": 0},
            {"map": "DeepDesert_1",    "dedicatedScaling": true}
          ]
        }
      }
    }
  }
}`

func TestListSets_ParsesEverySet(t *testing.T) {
	t.Parallel()
	sets, err := ListSets([]byte(setsSampleCR))
	if err != nil {
		t.Fatalf("ListSets err = %v", err)
	}
	if len(sets) != 5 {
		t.Fatalf("len(sets) = %d, want 5", len(sets))
	}
	want := []string{"Survival_1", "Overmap", "SH_Arrakeen", "SH_HarkoVillage", "DeepDesert_1"}
	for i, s := range sets {
		if s.Index != i {
			t.Errorf("sets[%d].Index = %d, want %d", i, s.Index, i)
		}
		if s.Map != want[i] {
			t.Errorf("sets[%d].Map = %q, want %q", i, s.Map, want[i])
		}
	}
	// SH_Arrakeen has no replicas field at all — Replicas must be nil.
	if sets[2].Replicas != nil {
		t.Errorf("SH_Arrakeen.Replicas = %v, want nil (field absent in CR)", *sets[2].Replicas)
	}
	// SH_HarkoVillage has "replicas: 0" explicitly — Replicas must be non-nil and 0.
	if sets[3].Replicas == nil || *sets[3].Replicas != 0 {
		t.Errorf("SH_HarkoVillage.Replicas = %v, want pointer to 0", sets[3].Replicas)
	}
	if !slices.Equal(sets[0].Partitions, []int{1}) {
		t.Errorf("Survival_1.Partitions = %v, want [1]", sets[0].Partitions)
	}
}

// partitionsByMap is the lookup the caller resolves from dune.world_partition
// before calling BuildScalingPatches. Tests reflect the live mapping observed
// on vm-host-02 / vm-host-00 (see project_funcom-bootstrap-quirks.md).
var partitionsByMap = map[string]int{
	"Survival_1":      1,
	"Overmap":         2,
	"SH_Arrakeen":     3,
	"SH_HarkoVillage": 4,
	"DeepDesert_1":    8,
}

func TestBuildScalingPatches_EnableMatchesIssueLiveBehaviour(t *testing.T) {
	t.Parallel()
	// Mirrors the live patch we ran for the user: enable SH_Arrakeen,
	// SH_HarkoVillage, DeepDesert_1 (dedicatedScaling=false, replicas=1,
	// partitions=[N] resolved from dune.world_partition).
	ops, notFound, err := BuildScalingPatches([]byte(setsSampleCR),
		[]string{"SH_Arrakeen", "SH_HarkoVillage", "DeepDesert_1"}, false, 1, partitionsByMap)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(notFound) != 0 {
		t.Errorf("notFound = %v, want empty", notFound)
	}

	wantOps := []Operation{
		{Op: "replace", Path: "/spec/serverGroup/template/spec/sets/2/dedicatedScaling", Value: false},
		{Op: "add", Path: "/spec/serverGroup/template/spec/sets/2/replicas", Value: 1},
		{Op: "add", Path: "/spec/serverGroup/template/spec/sets/2/partitions", Value: []int{3}},
		{Op: "replace", Path: "/spec/serverGroup/template/spec/sets/3/dedicatedScaling", Value: false},
		{Op: "replace", Path: "/spec/serverGroup/template/spec/sets/3/replicas", Value: 1},
		{Op: "add", Path: "/spec/serverGroup/template/spec/sets/3/partitions", Value: []int{4}},
		{Op: "replace", Path: "/spec/serverGroup/template/spec/sets/4/dedicatedScaling", Value: false},
		{Op: "add", Path: "/spec/serverGroup/template/spec/sets/4/replicas", Value: 1},
		{Op: "add", Path: "/spec/serverGroup/template/spec/sets/4/partitions", Value: []int{8}},
	}
	if !equalOps(ops, wantOps) {
		t.Errorf("ops:\n got  %+v\n want %+v", ops, wantOps)
	}
}

func TestBuildScalingPatches_DisableEmitsExpectedOps(t *testing.T) {
	t.Parallel()
	// Disabling Survival_1 (currently always-on with partitions=[1]) must
	// emit dedicatedScaling, replicas, AND a partitions-remove so the set
	// goes back to the Funcom-template shape.
	ops, _, err := BuildScalingPatches([]byte(setsSampleCR), []string{"Survival_1"}, true, 0, partitionsByMap)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	wantOps := []Operation{
		{Op: "replace", Path: "/spec/serverGroup/template/spec/sets/0/dedicatedScaling", Value: true},
		{Op: "replace", Path: "/spec/serverGroup/template/spec/sets/0/replicas", Value: 0},
		{Op: "remove", Path: "/spec/serverGroup/template/spec/sets/0/partitions"},
	}
	if !equalOps(ops, wantOps) {
		t.Errorf("ops:\n got  %+v\n want %+v", ops, wantOps)
	}
}

func TestBuildScalingPatches_NoChangeProducesNoOps(t *testing.T) {
	t.Parallel()
	// Survival_1 is already dedicatedScaling=false, replicas=1, partitions=[1].
	ops, _, err := BuildScalingPatches([]byte(setsSampleCR), []string{"Survival_1"}, false, 1, partitionsByMap)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(ops) != 0 {
		t.Errorf("ops = %+v, want empty (Survival_1 already in target state)", ops)
	}
}

func TestBuildScalingPatches_UnknownMapsReportedNotApplied(t *testing.T) {
	t.Parallel()
	ops, notFound, err := BuildScalingPatches([]byte(setsSampleCR),
		[]string{"SH_Arrakeen", "DoesNotExist", "AlsoMissing"}, false, 1, partitionsByMap)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !slices.Equal(notFound, []string{"DoesNotExist", "AlsoMissing"}) {
		t.Errorf("notFound = %v, want [DoesNotExist AlsoMissing]", notFound)
	}
	// Still produced ops for the one valid map.
	if len(ops) == 0 {
		t.Errorf("ops = empty, want patches for SH_Arrakeen")
	}
}

func TestBuildScalingPatches_DuplicateMapDeduped(t *testing.T) {
	t.Parallel()
	ops, _, err := BuildScalingPatches([]byte(setsSampleCR),
		[]string{"SH_Arrakeen", "SH_Arrakeen"}, false, 1, partitionsByMap)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	// SH_Arrakeen has neither replicas nor partitions → "add" for both.
	// Plus the dedicatedScaling replace. Exactly 3 ops total.
	if len(ops) != 3 {
		t.Errorf("len(ops) = %d, want 3 (duplicate map names must dedupe)", len(ops))
	}
}

func TestBuildScalingPatches_PartialReplicasUpdate(t *testing.T) {
	t.Parallel()
	// SH_HarkoVillage stays dedicatedScaling=true (on-demand). Bumping
	// replicas to 2 keeps the set on-demand, so partitions must not be
	// added (the set still has no partitions field, target wants none either).
	ops, _, err := BuildScalingPatches([]byte(setsSampleCR), []string{"SH_HarkoVillage"}, true, 2, partitionsByMap)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	wantOps := []Operation{
		{Op: "replace", Path: "/spec/serverGroup/template/spec/sets/3/replicas", Value: 2},
	}
	if !equalOps(ops, wantOps) {
		t.Errorf("ops:\n got  %+v\n want %+v", ops, wantOps)
	}
}

func TestBuildScalingPatches_EnableMissingPartitionLookupIsError(t *testing.T) {
	t.Parallel()
	// Enabling a map with no partitions[] field and no entry in the
	// supplied partitionByMap is a programming error — must fail loudly
	// rather than emit a half-patch that crashes the pod at runtime.
	_, _, err := BuildScalingPatches([]byte(setsSampleCR),
		[]string{"SH_Arrakeen"}, false, 1, map[string]int{})
	if err == nil {
		t.Fatalf("err = nil, want error about missing partition id")
	}
}

func equalOps(a, b []Operation) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Op != b[i].Op || a[i].Path != b[i].Path {
			return false
		}
		// Compare Value via string-ifying for primitives — good enough for
		// our shape (bool / int).
		if !valuesEqual(a[i].Value, b[i].Value) {
			return false
		}
	}
	return true
}

func valuesEqual(x, y any) bool {
	switch xv := x.(type) {
	case bool:
		yv, ok := y.(bool)
		return ok && xv == yv
	case int:
		yv, ok := y.(int)
		return ok && xv == yv
	case []int:
		yv, ok := y.([]int)
		return ok && slices.Equal(xv, yv)
	case nil:
		return y == nil
	default:
		return x == y
	}
}
