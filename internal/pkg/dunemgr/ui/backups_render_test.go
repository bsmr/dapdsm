package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"go.muehmer.eu/dapdsm/pkg/domain/store"
)

func TestBackupsPartialRendersTable(t *testing.T) {
	r := New()
	var buf bytes.Buffer
	rows := []store.BackupRecord{{
		Host: "vm-a", BG: "bg", Name: "weekly",
		UnixTS: 1717000000, Bytes: 4096,
		CreatedAt: time.Unix(1717000000, 0).UTC(),
	}}
	data := map[string]any{
		"Host":    "vm-a",
		"BG":      "bg",
		"Backups": rows,
	}
	if err := r.RenderPartial(&buf, "backups", data); err != nil {
		t.Fatalf("RenderPartial: %v", err)
	}
	body := buf.String()
	for _, want := range []string{
		"weekly", "vm-a", "bg", "Backups", `hx-post="/host/vm-a/backups"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q: %s", want, body)
		}
	}
}

func TestBackupsPartialEmptyState(t *testing.T) {
	r := New()
	var buf bytes.Buffer
	data := map[string]any{"Host": "vm-a", "BG": "bg", "Backups": []store.BackupRecord(nil)}
	if err := r.RenderPartial(&buf, "backups", data); err != nil {
		t.Fatalf("RenderPartial: %v", err)
	}
	if !strings.Contains(buf.String(), "no backups") {
		t.Errorf("missing empty-state message: %s", buf.String())
	}
}
