package command

import (
	"strings"
	"testing"
)

func TestSpecsCoverAllDispatcherVerbs(t *testing.T) {
	got := map[string]bool{}
	for _, s := range Specs() {
		got[s.Verb] = true
	}
	for _, v := range []string{"host", "lifecycle", "broadcast", "db", "backup", "shutdown", "avatar", "admin"} {
		if !got[v] {
			t.Errorf("Specs() missing verb %q", v)
		}
		if !Known(v) {
			t.Errorf("verb %q in table but not Known()", v)
		}
	}
	// Specs and the dispatch table must agree exactly (no drift).
	if len(Specs()) != len(table) {
		t.Errorf("Specs() has %d entries, dispatch table has %d", len(Specs()), len(table))
	}
}

func TestSpecForLifecycleHasHostThenActions(t *testing.T) {
	s, ok := SpecFor("lifecycle")
	if !ok {
		t.Fatal("SpecFor(lifecycle) not found")
	}
	if len(s.Args) < 2 {
		t.Fatalf("lifecycle should have >=2 arg slots, got %d", len(s.Args))
	}
	if s.Args[0].kind != argHost {
		t.Errorf("lifecycle arg[0] kind = %v, want argHost", s.Args[0].kind)
	}
	if s.Args[1].kind != argFixed {
		t.Errorf("lifecycle arg[1] kind = %v, want argFixed", s.Args[1].kind)
	}
	want := map[string]bool{"start": true, "stop": true, "restart": true, "update": true}
	for _, o := range s.Args[1].options {
		delete(want, o)
	}
	if len(want) != 0 {
		t.Errorf("lifecycle actions missing: %v", want)
	}
}

func TestSpecForUnknownReturnsFalse(t *testing.T) {
	if _, ok := SpecFor("frobnicate"); ok {
		t.Error("SpecFor(frobnicate) returned ok=true")
	}
}

func TestCandidatesByPosition(t *testing.T) {
	s, _ := SpecFor("lifecycle")
	if got := s.Candidates(0, []string{"vm-a", "vm-b"}); len(got) != 2 {
		t.Errorf("lifecycle pos0 (host) = %v, want 2 hosts", got)
	}
	if got := s.Candidates(1, nil); len(got) != 4 {
		t.Errorf("lifecycle pos1 (actions) = %v, want 4", got)
	}
	if got := s.Candidates(5, nil); got != nil {
		t.Errorf("out-of-range pos = %v, want nil", got)
	}
	db, _ := SpecFor("db")
	if got := db.Candidates(2, nil); got != nil { // exec's next slot is argFree
		t.Errorf("db freeform slot = %v, want nil", got)
	}
}

func TestAvatarSpecShape(t *testing.T) {
	s, ok := SpecFor("avatar")
	if !ok {
		t.Fatal("no avatar spec")
	}
	if len(s.Args) != 4 {
		t.Fatalf("want 4 arg slots, got %d", len(s.Args))
	}
	if s.Args[0].kind != argFixed {
		t.Fatal("slot 0 should be the fixed sub-verb")
	}
	wantSubs := map[string]bool{"export": false, "list": false, "import": false, "transfer": false}
	for _, o := range s.Args[0].options {
		if _, ok := wantSubs[o]; ok {
			wantSubs[o] = true
		}
	}
	for sub, seen := range wantSubs {
		if !seen {
			t.Fatalf("sub-verb %q missing from spec options %v", sub, s.Args[0].options)
		}
	}
	if s.Args[1].kind != argHost || s.Args[2].kind != argHost {
		t.Fatal("slots 1 and 2 should be argHost (src/dst for transfer)")
	}
}

func TestAvatarUsageString(t *testing.T) {
	s, ok := SpecFor("avatar")
	if !ok {
		t.Fatal("no avatar spec")
	}
	got := s.Usage()
	if !strings.Contains(got, "avatar") {
		t.Fatalf("usage missing verb: %q", got)
	}
}
