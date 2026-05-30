package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestLifecyclePartialRenders(t *testing.T) {
	r := New()
	var buf bytes.Buffer
	data := map[string]any{"Host": "vm-a"}
	if err := r.RenderPartial(&buf, "lifecycle", data); err != nil {
		t.Fatalf("RenderPartial: %v", err)
	}
	body := buf.String()
	for _, want := range []string{
		"start", "stop", "restart", "update", "vm-a",
		`hx-post="/host/vm-a/lifecycle/start"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in render: %s", want, body)
		}
	}
}
