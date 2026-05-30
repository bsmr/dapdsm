package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
)

func TestDashboardHasTabNav(t *testing.T) {
	r := New()
	data := struct {
		Host       string
		BG         string
		Snap       store.StatusSnapshot
		LastAction *store.AuditEntry
	}{
		Host: "vm-a",
		BG:   "myBG",
		LastAction: &store.AuditEntry{
			TS:       time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC),
			Operator: "local",
			Action:   "lifecycle.start",
			Result:   "ok",
		},
	}
	var buf bytes.Buffer
	if err := r.Render(&buf, "dashboard", data); err != nil {
		t.Fatalf("Render dashboard: %v", err)
	}
	body := buf.String()
	for _, want := range []string{
		`hx-get="/host/vm-a/lifecycle/_partial"`,
		`hx-get="/host/vm-a/backups?bg=myBG"`,
		`hx-get="/host/vm-a/shutdown/_partial"`,
		`hx-get="/host/vm-a/db"`,
		`hx-get="/audit"`,
		"lifecycle.start",
		"local",
		`data-host="vm-a"`,
		`id="bg-state"`,
		`id="pod-count"`,
		`id="health-badge"`,
		`id="bg-error"`,
		`id="tab-spinner"`,
		`hx-indicator="#tab-spinner"`,
		`id="last-action"`,
		`aria-label="breadcrumb"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in dashboard: %s", want, body)
		}
	}
}
