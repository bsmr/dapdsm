package tui

import (
	"strings"
	"testing"
)

func cols() []navColumn {
	return []navColumn{
		{"ID", cellLeft, true},
		{"NAME", cellLeft, true},
		{"STACK", cellRight, true},
		{"MAX", cellRight, false},
		{"Q", cellLeft, true},
	}
}

func TestFormatTableAlignsHeaderAndRows(t *testing.T) {
	cells := [][]string{
		{"id=1", "Fuel Cell", "44", "500", "q0"},
		{"id=222", "healthpack_channeled", "14", "", "q0"},
		{"id=3", "Light Darts", "1000", "1000", "q3"},
	}
	header, rows := formatTable(cols(), cells, -1, false)
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	off := strings.Index(header, "NAME")
	for i, want := range []string{"Fuel Cell", "healthpack_channeled", "Light Darts"} {
		if got := strings.Index(rows[i], want); got != off {
			t.Fatalf("row %d NAME offset %d != header %d\n%q\n%q", i, got, off, header, rows[i])
		}
	}
	// STACK right-aligned: "44" and "1000" end at the same column.
	end := func(s, sub string) int { return strings.Index(s, sub) + len(sub) }
	if end(rows[0], "44") != end(rows[2], "1000") {
		t.Fatalf("STACK not right-aligned:\n%q\n%q", rows[0], rows[2])
	}
	// blank MAX: row 1 must not render a slash or a spurious max digit.
	if strings.Contains(rows[1], "/") {
		t.Fatalf("blank max must not render a slash: %q", rows[1])
	}
}

func TestFormatTableSortMarker(t *testing.T) {
	c := cols()
	cells := [][]string{{"id=1", "A", "1", "", "q0"}}
	h, _ := formatTable(c, cells, 1, false)
	if !strings.Contains(h, "NAME^") {
		t.Fatalf("ascending marker missing: %q", h)
	}
	h, _ = formatTable(c, cells, 1, true)
	if !strings.Contains(h, "NAMEv") {
		t.Fatalf("descending marker missing: %q", h)
	}
	h, _ = formatTable(c, cells, 3, false) // MAX not sortable
	if strings.Contains(h, "MAX^") || strings.Contains(h, "MAXv") {
		t.Fatalf("non-sortable column must not be marked: %q", h)
	}
}

func TestFormatTableEmpty(t *testing.T) {
	h, rows := formatTable(cols(), nil, 1, false)
	if h == "" || len(rows) != 0 {
		t.Fatalf("empty cells → header only, got header=%q rows=%d", h, len(rows))
	}
}
