package dbquery

import (
	"context"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

func TestTablesParsesRows(t *testing.T) {
	rr := &pipingRunner{
		podStdout: fakeDBDeploy,
		mqStdout:  "dune|player\ndune|world_partition\npublic|migrations\n",
	}
	st := newTempStore(t)
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: st}
	got, err := r.Tables(context.Background(), "vm-a")
	if err != nil {
		t.Fatalf("Tables: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("rows=%d, want 3", len(got))
	}
	if got[0].Schema != "dune" || got[0].Name != "player" {
		t.Errorf("row[0]=%+v", got[0])
	}
	// No audit entries — Tables is a framework read.
	entries, _ := st.ListAudit(0)
	if len(entries) != 0 {
		t.Errorf("Tables wrote audit entries: %+v", entries)
	}
}

func TestColumnsParsesRows(t *testing.T) {
	rr := &pipingRunner{
		podStdout: fakeDBDeploy,
		mqStdout:  "id|bigint\nname|text\n",
	}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	got, err := r.Columns(context.Background(), "vm-a", "dune", "player")
	if err != nil {
		t.Fatalf("Columns: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("cols=%d, want 2", len(got))
	}
	if got[0].Name != "id" || got[0].Type != "bigint" {
		t.Errorf("col[0]=%+v", got[0])
	}
}

func TestColumnsRejectsInvalidIdentifiers(t *testing.T) {
	r := &Runner{SSH: &ssh.Client{Runner: &pipingRunner{}}, Store: newTempStore(t)}
	cases := [][2]string{
		{"dune'; DROP", "player"},
		{"dune", "player; --"},
		{"", "player"},
		{"dune", ""},
	}
	for _, c := range cases {
		if _, err := r.Columns(context.Background(), "vm-a", c[0], c[1]); err == nil {
			t.Errorf("Columns(schema=%q, table=%q) err=nil, want non-nil", c[0], c[1])
		}
	}
}

func TestSchemaResultsTrimmedOfBlankLines(t *testing.T) {
	rr := &pipingRunner{
		podStdout: fakeDBDeploy,
		mqStdout:  "\ndune|player\n\n",
	}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	got, _ := r.Tables(context.Background(), "vm-a")
	if len(got) != 1 {
		t.Errorf("rows=%d, want 1 (blank lines filtered): %+v", len(got), got)
	}
}
