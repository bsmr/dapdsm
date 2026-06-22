package battlegroup

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// fakeRunner captures Patch calls and serves canned Get output.
type fakeRunner struct {
	getOut    []byte
	getErr    error
	patches   []patchCall
	patchErr  error
}
type patchCall struct{ resource, name, ns, ptype, body string }

func (f *fakeRunner) Get(ctx context.Context, args ...string) ([]byte, error) {
	return f.getOut, f.getErr
}
func (f *fakeRunner) Patch(ctx context.Context, resource, name, ns, ptype, body string) error {
	f.patches = append(f.patches, patchCall{resource, name, ns, ptype, body})
	return f.patchErr
}
func (f *fakeRunner) DeletePods(context.Context, string, ...string) error { return nil }
func (f *fakeRunner) Exec(context.Context, string, string, ...string) ([]byte, error) {
	return nil, nil
}
func (f *fakeRunner) ExecPiped(context.Context, string, string, []byte, ...string) ([]byte, error) {
	return nil, nil
}

func TestStopPatchesSpecStopTrue(t *testing.T) {
	r := &fakeRunner{}
	if err := Stop(context.Background(), r, "funcom-seabass-x", "x"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if len(r.patches) != 1 {
		t.Fatalf("want 1 patch, got %d", len(r.patches))
	}
	p := r.patches[0]
	if p.resource != "battlegroup" || p.name != "x" || p.ns != "funcom-seabass-x" || p.ptype != "merge" {
		t.Fatalf("bad patch target: %+v", p)
	}
	var got map[string]map[string]any
	if err := json.Unmarshal([]byte(p.body), &got); err != nil {
		t.Fatalf("patch body not JSON: %v (%s)", err, p.body)
	}
	if got["spec"]["stop"] != true {
		t.Fatalf("want spec.stop=true, got %v", got["spec"]["stop"])
	}
}

func TestStartPatchesSpecStopFalse(t *testing.T) {
	r := &fakeRunner{}
	if err := Start(context.Background(), r, "ns", "x"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	var got map[string]map[string]any
	_ = json.Unmarshal([]byte(r.patches[0].body), &got)
	if got["spec"]["stop"] != false {
		t.Fatalf("want spec.stop=false, got %v", got["spec"]["stop"])
	}
}

func TestRestartStopsThenStarts(t *testing.T) {
	r := &fakeRunner{}
	if err := Restart(context.Background(), r, "ns", "x"); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if len(r.patches) != 2 {
		t.Fatalf("want 2 patches, got %d", len(r.patches))
	}
	var first map[string]map[string]any
	_ = json.Unmarshal([]byte(r.patches[0].body), &first)
	if first["spec"]["stop"] != true {
		t.Fatalf("first patch must stop, got %v", first["spec"]["stop"])
	}
}

func TestUpdateReconcilesPlaceholderTags(t *testing.T) {
	cr := []byte(`{"spec":{"a":{"image":"registry.funcom.com/x:0-0-shipping"}}}`)
	r := &fakeRunner{getOut: cr}
	if err := Update(context.Background(), r, "ns", "x", "1988751-0-shipping"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(r.patches) != 1 || r.patches[0].ptype != "json" {
		t.Fatalf("want one json patch, got %+v", r.patches)
	}
	if !strings.Contains(r.patches[0].body, "1988751-0-shipping") {
		t.Fatalf("tag not in patch body: %s", r.patches[0].body)
	}
}

func TestUpdateNoMatchingTagIsNoop(t *testing.T) {
	cr := []byte(`{"spec":{"a":{"image":"registry.funcom.com/x:already-real"}}}`)
	r := &fakeRunner{getOut: cr}
	if err := Update(context.Background(), r, "ns", "x", "1988751-0-shipping"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(r.patches) != 0 {
		t.Fatalf("expected no patch (nothing to reconcile), got %d", len(r.patches))
	}
}
