// internal/pkg/dunemgr/ui/dbextra_render_test.go
package ui

import (
	"bytes"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/domain/dbquery"
)

func TestDBColumnsPartialRenders(t *testing.T) {
	r := New()
	var buf bytes.Buffer
	if err := r.RenderPartial(&buf, "db_columns", map[string]any{
		"Host": "vm-a", "Schema": "dune", "Table": "player",
		"Columns": []dbquery.Column{{Name: "id", Type: "bigint"}},
	}); err != nil {
		t.Fatalf("RenderPartial: %v", err)
	}
	body := buf.String()
	for _, want := range []string{"dune", "player", "id", "bigint"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q: %s", want, body)
		}
	}
}

func TestDBSlowPartialRenders(t *testing.T) {
	r := New()
	var buf bytes.Buffer
	if err := r.RenderPartial(&buf, "db_slow", map[string]any{
		"Host": "vm-a",
		"Rows": []dbquery.SlowQuery{{MeanMS: 120.5, Calls: 42, Query: "SELECT 1"}},
	}); err != nil {
		t.Fatalf("RenderPartial: %v", err)
	}
	if !strings.Contains(buf.String(), "SELECT 1") {
		t.Errorf("missing query text: %s", buf.String())
	}
}

func TestDBSlowPartialEmpty(t *testing.T) {
	r := New()
	var buf bytes.Buffer
	_ = r.RenderPartial(&buf, "db_slow", map[string]any{"Host": "vm-a", "Rows": []dbquery.SlowQuery(nil)})
	if !strings.Contains(buf.String(), "no slow queries") {
		t.Errorf("missing empty state: %s", buf.String())
	}
}
