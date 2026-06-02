package tui

import "testing"

func TestInjectHostAddsSelectedWhenAbsent(t *testing.T) {
	hosts := []string{"vm-dune-01", "vm-dune-02"}
	got := injectHost("player inspect Stilgar", "vm-dune-01", hosts)
	if got != "player vm-dune-01 inspect Stilgar" {
		t.Fatalf("got %q", got)
	}
}

func TestInjectHostKeepsExplicitAlias(t *testing.T) {
	hosts := []string{"vm-dune-01", "vm-dune-02"}
	got := injectHost("player vm-dune-02 inspect Stilgar", "vm-dune-01", hosts)
	if got != "player vm-dune-02 inspect Stilgar" {
		t.Fatalf("explicit host must win: %q", got)
	}
}

func TestInjectHostSkipsNonHostVerb(t *testing.T) {
	got := injectHost("help", "vm-dune-01", []string{"vm-dune-01"})
	if got != "help" {
		t.Fatalf("non-host verb unchanged: %q", got)
	}
}

func TestInjectHostNoSelection(t *testing.T) {
	got := injectHost("player inspect Stilgar", "", []string{"vm-dune-01"})
	if got != "player inspect Stilgar" {
		t.Fatalf("no selection → unchanged: %q", got)
	}
}

func TestInjectHostVerbOnly(t *testing.T) {
	got := injectHost("stats", "vm-dune-01", []string{"vm-dune-01"})
	if got != "stats vm-dune-01" {
		t.Fatalf("verb-only argHost should append host: %q", got)
	}
}
