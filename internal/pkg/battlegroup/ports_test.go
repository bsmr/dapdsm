package battlegroup

import (
	"testing"
)

const portsSampleCR = `{
  "spec": {
    "serverGroup": {
      "template": {
        "spec": {
          "sets": [
            {"map": "Survival_1", "arguments": ["-FarmRegion=Europe", "-RMQGameTlsEnabled=true"]},
            {"map": "Overmap",    "arguments": ["-FarmRegion=Europe", "-ini:engine:[URL]:Port=7877", "-ini:engine:[URL]:IGWPort=7988"]},
            {"map": "SH_Arrakeen","arguments": ["-FarmRegion=Europe", "-ini:engine:[URL]:Port=7000", "-ini:engine:[URL]:IGWPort=8000"]},
            {"map": "DeepDesert_1"}
          ]
        }
      }
    }
  }
}`

func TestBuildPortPatches_AddsWhenMissing(t *testing.T) {
	t.Parallel()
	// Survival_1 (idx 0) has neither arg → 2× add expected.
	// DeepDesert_1 (idx 3) has no arguments field at all → add ops still
	// reference /arguments/-, which Funcom-Operator silently materialises;
	// to be safe the patch should first add the arguments array if missing.
	ops, err := BuildPortPatches([]byte(portsSampleCR), 7877, 7988)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	// Look for the additions for set index 0 (Survival_1).
	addedPort, addedIGW := false, false
	for _, op := range ops {
		if op.Op == "add" && op.Path == "/spec/serverGroup/template/spec/sets/0/arguments/-" {
			if v, _ := op.Value.(string); v == "-ini:engine:[URL]:Port=7877" {
				addedPort = true
			}
			if v, _ := op.Value.(string); v == "-ini:engine:[URL]:IGWPort=7988" {
				addedIGW = true
			}
		}
	}
	if !addedPort || !addedIGW {
		t.Errorf("missing add op for Survival_1: gamePort=%v igwPort=%v\n  ops: %+v", addedPort, addedIGW, ops)
	}
}

func TestBuildPortPatches_NoOpWhenCorrect(t *testing.T) {
	t.Parallel()
	// Overmap (idx 1) already has Port=7877 and IGWPort=7988 → no ops
	// emitted for index 1.
	ops, err := BuildPortPatches([]byte(portsSampleCR), 7877, 7988)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	for _, op := range ops {
		if op.Path == "/spec/serverGroup/template/spec/sets/1/arguments/-" ||
			(op.Op == "replace" && containsSetIndex(op.Path, 1)) {
			t.Errorf("unexpected op for Overmap (already correct): %+v", op)
		}
	}
}

func TestBuildPortPatches_ReplacesWhenDifferent(t *testing.T) {
	t.Parallel()
	// SH_Arrakeen (idx 2) has Port=7000 and IGWPort=8000; target is 7877/7988
	// → expect 2× replace operations at the existing positions, NOT add.
	ops, err := BuildPortPatches([]byte(portsSampleCR), 7877, 7988)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	gotPortReplace, gotIGWReplace := false, false
	for _, op := range ops {
		if op.Op != "replace" || !containsSetIndex(op.Path, 2) {
			continue
		}
		if v, _ := op.Value.(string); v == "-ini:engine:[URL]:Port=7877" {
			gotPortReplace = true
		}
		if v, _ := op.Value.(string); v == "-ini:engine:[URL]:IGWPort=7988" {
			gotIGWReplace = true
		}
	}
	if !gotPortReplace || !gotIGWReplace {
		t.Errorf("SH_Arrakeen: expected replace ops for both port keys, got port=%v igw=%v\n  ops: %+v",
			gotPortReplace, gotIGWReplace, ops)
	}
}

func TestBuildPortPatches_RejectsNonPositivePorts(t *testing.T) {
	t.Parallel()
	if _, err := BuildPortPatches([]byte(portsSampleCR), 0, 7988); err == nil {
		t.Error("err = nil for gameBase=0, want error")
	}
	if _, err := BuildPortPatches([]byte(portsSampleCR), 7877, -1); err == nil {
		t.Error("err = nil for igwBase=-1, want error")
	}
}

// containsSetIndex reports whether a JSON-Pointer path targets the
// given set index — covers both `/sets/<i>` and `/sets/<i>/arguments/...`.
func containsSetIndex(path string, idx int) bool {
	prefix := "/spec/serverGroup/template/spec/sets/" + itoa(idx)
	if len(path) < len(prefix) {
		return false
	}
	if path[:len(prefix)] != prefix {
		return false
	}
	// Next char must be '/' or end-of-string so we don't match /sets/10 for idx 1.
	if len(path) == len(prefix) {
		return true
	}
	return path[len(prefix)] == '/'
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}
