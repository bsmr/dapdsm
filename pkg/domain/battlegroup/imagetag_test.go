package battlegroup

import "testing"

func TestBuildImageTagPatches(t *testing.T) {
	cr := []byte(`{"spec":{"a":{"image":"registry.funcom.com/seabass-server-gateway:0-0-shipping"},
		"b":{"image":"registry.funcom.com/seabass-director:0-0-shipping"},
		"c":{"image":"busybox:1.36"}}}`)
	ops, err := BuildImageTagPatches(cr, "1988751-0-shipping")
	if err != nil {
		t.Fatalf("BuildImageTagPatches: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("want 2 replace ops, got %d: %+v", len(ops), ops)
	}
	for _, op := range ops {
		if op.Op != "replace" {
			t.Errorf("op = %q, want replace", op.Op)
		}
		v, _ := op.Value.(string)
		if v != "registry.funcom.com/seabass-server-gateway:1988751-0-shipping" &&
			v != "registry.funcom.com/seabass-director:1988751-0-shipping" {
			t.Errorf("unexpected replace value %q at %s", v, op.Path)
		}
	}
	// busybox (no placeholder tag) is untouched → no op references it.
}

func TestBuildImageTagPatches_Idempotent(t *testing.T) {
	cr := []byte(`{"x":{"image":"registry.funcom.com/seabass-director:1988751-0-shipping"}}`)
	ops, err := BuildImageTagPatches(cr, "1988751-0-shipping")
	if err != nil {
		t.Fatalf("BuildImageTagPatches: %v", err)
	}
	if len(ops) != 0 {
		t.Errorf("already-reconciled CR produced %d ops, want 0", len(ops))
	}
}
