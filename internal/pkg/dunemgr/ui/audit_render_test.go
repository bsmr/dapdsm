package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
)

func TestAuditPartialRendersRows(t *testing.T) {
	r := New()
	var buf bytes.Buffer
	entries := []store.AuditEntry{
		{
			TS:       time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC),
			Operator: "local",
			Host:     "vm-a",
			Action:   "lifecycle.start",
			Result:   "ok",
		},
		{
			TS:       time.Date(2026, 5, 29, 11, 0, 0, 0, time.UTC),
			Operator: "local",
			Host:     "vm-a",
			Action:   "db.exec",
			Subject:  "SELECT 1",
			Result:   "ok",
		},
	}
	if err := r.RenderPartial(&buf, "audit", map[string]any{
		"Entries": entries,
		"Offset":  0,
		"Limit":   50,
	}); err != nil {
		t.Fatalf("RenderPartial: %v", err)
	}
	body := buf.String()
	for _, want := range []string{"lifecycle.start", "db.exec", "vm-a", "local"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q: %s", want, body)
		}
	}
}

func TestAuditPartialEmpty(t *testing.T) {
	r := New()
	var buf bytes.Buffer
	_ = r.RenderPartial(&buf, "audit", map[string]any{
		"Entries": []store.AuditEntry(nil),
		"Offset":  0,
		"Limit":   50,
	})
	if !strings.Contains(buf.String(), "no audit entries") {
		t.Errorf("missing empty-state: %s", buf.String())
	}
}
