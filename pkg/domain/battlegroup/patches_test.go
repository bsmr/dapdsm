package battlegroup

import (
	"encoding/json"
	"slices"
	"testing"
)

const sampleCR = `{
  "spec": {
    "utilities": {
      "director": {
        "spec": {
          "envVars": [
            {"name": "OTHER", "value": "x"},
            {"name": "HOST_DATACENTER_IP_ADDRESS", "value": "127.0.0.1"}
          ]
        }
      },
      "serverGateway": {
        "spec": {
          "envVars": [
            {"name": "HOST_DATACENTER_IP_ADDRESS", "value": "127.0.0.1"},
            {"name": "OTHER", "value": "y"}
          ]
        }
      },
      "textRouter": {
        "spec": {
          "envVars": [
            {"name": "HOST_DATACENTER_IP_ADDRESS"},
            {"name": "OTHER", "value": "z"}
          ]
        }
      }
    },
    "serverGroup": {
      "template": {
        "spec": {
          "sets": [
            {"schedulerName": "memory-focused-scheduler", "name": "set0"},
            {"name": "set1"},
            {"schedulerName": "memory-focused-scheduler", "name": "set2"},
            {"schedulerName": "default-scheduler", "name": "set3"}
          ]
        }
      }
    }
  }
}`

func TestBuildHostIPPatches_FindsAllOccurrences(t *testing.T) {
	t.Parallel()
	ops, err := BuildHostIPPatches([]byte(sampleCR), "203.0.113.42")
	if err != nil {
		t.Fatalf("BuildHostIPPatches() err = %v", err)
	}
	paths := opPaths(ops)
	wantPaths := []string{
		"/spec/utilities/director/spec/envVars/1/value",
		"/spec/utilities/serverGateway/spec/envVars/0/value",
		"/spec/utilities/textRouter/spec/envVars/0/value",
	}
	slices.Sort(paths)
	slices.Sort(wantPaths)
	if !slices.Equal(paths, wantPaths) {
		t.Errorf("paths = %v, want %v", paths, wantPaths)
	}

	for _, op := range ops {
		if op.Value != "203.0.113.42" {
			t.Errorf("op %s value = %v, want %q", op.Path, op.Value, "203.0.113.42")
		}
	}
}

func TestBuildHostIPPatches_UsesAddForEntriesWithoutValue(t *testing.T) {
	t.Parallel()
	ops, err := BuildHostIPPatches([]byte(sampleCR), "203.0.113.42")
	if err != nil {
		t.Fatalf("BuildHostIPPatches() err = %v", err)
	}
	for _, op := range ops {
		if op.Path == "/spec/utilities/textRouter/spec/envVars/0/value" {
			if op.Op != "add" {
				t.Errorf("textRouter op = %q, want %q (value key was absent)", op.Op, "add")
			}
			return
		}
	}
	t.Fatalf("textRouter operation not generated")
}

func TestBuildHostIPPatches_IsIdempotent(t *testing.T) {
	t.Parallel()
	ops, err := BuildHostIPPatches([]byte(sampleCR), "127.0.0.1")
	if err != nil {
		t.Fatalf("BuildHostIPPatches() err = %v", err)
	}
	// director and serverGateway already carry 127.0.0.1; textRouter has no
	// value yet, so only that one should still be patched (as add).
	if len(ops) != 1 || ops[0].Op != "add" {
		t.Errorf("ops = %+v, want exactly one add-op for textRouter", ops)
	}
}

func TestBuildHostIPPatches_AllSpecialCharsInPathAreEscaped(t *testing.T) {
	t.Parallel()
	input := `{"spec":{"weird~key/with":[{"envVars":[{"name":"HOST_DATACENTER_IP_ADDRESS","value":"127.0.0.1"}]}]}}`
	ops, err := BuildHostIPPatches([]byte(input), "1.2.3.4")
	if err != nil {
		t.Fatalf("BuildHostIPPatches() err = %v", err)
	}
	want := "/spec/weird~0key~1with/0/envVars/0/value"
	if len(ops) != 1 || ops[0].Path != want {
		t.Errorf("path = %q, want %q", ops[0].Path, want)
	}
}

const sampleCRWithIDs = `{
  "spec": {
    "utilities": {
      "director": {"spec": {"envVars": [
        {"name": "HOST_DATACENTER_ID", "value": "dune-testing"},
        {"name": "HOST_DATACENTER_IP_ADDRESS", "value": "127.0.0.1"}
      ]}},
      "serverGateway": {"spec": {"envVars": [
        {"name": "HOST_DATACENTER_ID", "value": "dune-testing"}
      ]}}
    }
  }
}`

func TestBuildHostIDPatches_ReplacesEveryOccurrence(t *testing.T) {
	t.Parallel()
	ops, err := BuildHostIDPatches([]byte(sampleCRWithIDs), "vm-host-01")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(ops) != 2 {
		t.Errorf("len(ops) = %d, want 2 (director + serverGateway)", len(ops))
	}
	for _, op := range ops {
		if op.Op != "replace" || op.Value != "vm-host-01" {
			t.Errorf("unexpected op: %+v", op)
		}
	}
}

func TestBuildHostIDPatches_EmptyValueIsNoOp(t *testing.T) {
	t.Parallel()
	ops, err := BuildHostIDPatches([]byte(sampleCRWithIDs), "")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(ops) != 0 {
		t.Errorf("empty newID should produce no ops; got %d", len(ops))
	}
}

func TestBuildHostIDPatches_IdempotentWhenAlreadyMatching(t *testing.T) {
	t.Parallel()
	ops, err := BuildHostIDPatches([]byte(sampleCRWithIDs), "dune-testing")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(ops) != 0 {
		t.Errorf("ops = %+v, want empty (already in target state)", ops)
	}
}

func TestBuildHostIPPatches_RejectsMalformedJSON(t *testing.T) {
	t.Parallel()
	if _, err := BuildHostIPPatches([]byte("{not-json"), "1.2.3.4"); err == nil {
		t.Errorf("BuildHostIPPatches(malformed) err = nil, want error")
	}
}

func TestBuildSchedulerNameRemovals_DescendingOrder(t *testing.T) {
	t.Parallel()
	ops, err := BuildSchedulerNameRemovals([]byte(sampleCR))
	if err != nil {
		t.Fatalf("BuildSchedulerNameRemovals() err = %v", err)
	}
	wantPaths := []string{
		"/spec/serverGroup/template/spec/sets/2/schedulerName",
		"/spec/serverGroup/template/spec/sets/0/schedulerName",
	}
	gotPaths := opPaths(ops)
	if !slices.Equal(gotPaths, wantPaths) {
		t.Errorf("paths = %v, want %v (descending so sequential apply does not shift)", gotPaths, wantPaths)
	}
	for _, op := range ops {
		if op.Op != "remove" {
			t.Errorf("op = %q, want %q", op.Op, "remove")
		}
		if op.Value != nil {
			t.Errorf("remove op carries value %v, want nil", op.Value)
		}
	}
}

func TestBuildSchedulerNameRemovals_NoSetsField(t *testing.T) {
	t.Parallel()
	ops, err := BuildSchedulerNameRemovals([]byte(`{"spec":{}}`))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(ops) != 0 {
		t.Errorf("ops = %v, want empty", ops)
	}
}

// Operation must marshal to a "remove" without a "value" key so kubectl
// accepts it (RFC 6902 forbids value on remove).
func TestOperation_RemoveOmitsValueField(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(Operation{Op: "remove", Path: "/x"})
	if err != nil {
		t.Fatalf("marshal err = %v", err)
	}
	got := string(b)
	want := `{"op":"remove","path":"/x"}`
	if got != want {
		t.Errorf("Marshal = %q, want %q", got, want)
	}
}

func opPaths(ops []Operation) []string {
	out := make([]string, len(ops))
	for i, op := range ops {
		out[i] = op.Path
	}
	return out
}
