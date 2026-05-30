// internal/pkg/dunemgr/ui/db_render_test.go
package ui

import (
	"bytes"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/dbquery"
)

func TestDBPartialRenders(t *testing.T) {
	r := New()
	var buf bytes.Buffer
	data := map[string]any{
		"Host":   "vm-a",
		"Tables": []dbquery.Table{{Schema: "dune", Name: "player"}},
	}
	if err := r.RenderPartial(&buf, "db", data); err != nil {
		t.Fatalf("RenderPartial: %v", err)
	}
	body := buf.String()
	for _, want := range []string{
		"vm-a", "dune", "player",
		`hx-post="/host/vm-a/db/exec"`,
		`name="sql"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in render: %s", want, body)
		}
	}
}

func TestDBPartialEmptyTables(t *testing.T) {
	r := New()
	var buf bytes.Buffer
	if err := r.RenderPartial(&buf, "db", map[string]any{"Host": "vm-a", "Tables": []dbquery.Table(nil)}); err != nil {
		t.Fatalf("RenderPartial: %v", err)
	}
	if !strings.Contains(buf.String(), "no tables") {
		t.Errorf("missing empty-state: %s", buf.String())
	}
}
