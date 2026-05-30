package ui

import (
	"bytes"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
)

func TestShutdownPartialFormWhenIdle(t *testing.T) {
	r := New()
	var buf bytes.Buffer
	if err := r.RenderPartial(&buf, "shutdown", map[string]any{
		"Host": "vm-a", "Pending": false, "Rec": store.ScheduledShutdown{},
	}); err != nil {
		t.Fatalf("RenderPartial: %v", err)
	}
	body := buf.String()
	for _, want := range []string{
		`hx-post="/host/vm-a/shutdown"`, `name="lead"`, `name="action"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("idle render missing %q: %s", want, body)
		}
	}
}

func TestShutdownPartialShowsPending(t *testing.T) {
	r := New()
	var buf bytes.Buffer
	if err := r.RenderPartial(&buf, "shutdown", map[string]any{
		"Host": "vm-a", "Pending": true,
		"Rec": store.ScheduledShutdown{Host: "vm-a", Action: "stop", AtUnix: 1300},
	}); err != nil {
		t.Fatalf("RenderPartial: %v", err)
	}
	if !strings.Contains(buf.String(), `hx-post="/host/vm-a/shutdown/cancel"`) {
		t.Errorf("pending render missing cancel button: %s", buf.String())
	}
}
