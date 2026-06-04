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
	// host-first: Args has exactly 2 slots (host, fixed sub).
	if len(s.Args) != 2 {
		t.Fatalf("want 2 arg slots (host-first), got %d", len(s.Args))
	}
	if s.Args[0].kind != argHost {
		t.Fatal("slot 0 should be argHost (host-first)")
	}
	if s.Args[1].kind != argFixed {
		t.Fatal("slot 1 should be the fixed sub-verb")
	}
	// SubArgs must carry all 5 sub-verb keys.
	wantSubs := map[string]bool{"export": false, "list": false, "exports": false, "import": false, "transfer": false}
	for sub := range s.SubArgs {
		if _, ok := wantSubs[sub]; ok {
			wantSubs[sub] = true
		}
	}
	for sub, seen := range wantSubs {
		if !seen {
			t.Fatalf("sub-verb %q missing from SubArgs keys", sub)
		}
	}
	// argFixed options must list all 5 sub-verbs (catches missing options entry).
	wantOpts := map[string]bool{"export": false, "list": false, "exports": false, "import": false, "transfer": false}
	for _, o := range s.Args[1].options {
		wantOpts[o] = true
	}
	for o, seen := range wantOpts {
		if !seen {
			t.Fatalf("sub-verb %q missing from argFixed options", o)
		}
	}
}

func TestAvatarHostFirstSubArgs(t *testing.T) {
	a, _ := SpecFor("avatar")
	if !a.FirstArgIsHost() {
		t.Fatal("avatar must be host-first")
	}
	if !a.IsPlayerPos(2, "avatar", "vm-a", "export") {
		t.Error("avatar export name slot should be argPlayer")
	}
	if got := a.Candidates(2, []string{"vm-a", "vm-b"}, "avatar", "vm-a", "transfer"); len(got) != 2 {
		t.Errorf("avatar transfer dst-host slot should offer hosts, got %v", got)
	}
	if !a.IsPlayerPos(3, "avatar", "vm-a", "transfer") {
		t.Error("avatar transfer name slot (idx 3) should be argPlayer")
	}
	if !a.IsPlayerPos(2, "avatar", "vm-a", "import") {
		t.Error("avatar import name slot should be argPlayer")
	}
	if a.IsPlayerPos(3, "avatar", "vm-a", "import") {
		t.Error("avatar import key slot (idx 3) should be freeform")
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

// TestIsPlayerPos verifies that IsPlayerPos returns true for the whisper
// player slot and false for host, fixed, catalog, and free slots.
func TestIsPlayerPos(t *testing.T) {
	w, ok := SpecFor("whisper")
	if !ok {
		t.Fatal("no whisper spec")
	}
	// pos 0: host — not a player slot
	if w.IsPlayerPos(0) {
		t.Error("whisper pos0 (host) should not be a player slot")
	}
	// pos 1: player — must be argPlayer
	if !w.IsPlayerPos(1) {
		t.Error("whisper pos1 should be an argPlayer slot")
	}
	// pos 2: message (argFree) — not a player slot
	if w.IsPlayerPos(2) {
		t.Error("whisper pos2 (message) should not be a player slot")
	}
	// out of range
	if w.IsPlayerPos(-1) || w.IsPlayerPos(99) {
		t.Error("out-of-range pos should not be a player slot")
	}
}

// TestArgPlayerCandidatesNil verifies that the command package returns nil
// for argPlayer slots (the TUI supplies names from its live cache).
func TestArgPlayerCandidatesNil(t *testing.T) {
	w, _ := SpecFor("whisper")
	if got := w.Candidates(1, nil); got != nil {
		t.Errorf("argPlayer Candidates should be nil, got %v", got)
	}
}

// TestArgPlayerUsageRendered verifies that Usage() renders argPlayer as
// "<player>" (same style as argFree).
func TestArgPlayerUsageRendered(t *testing.T) {
	w, _ := SpecFor("whisper")
	got := w.Usage()
	if !strings.Contains(got, "<player>") {
		t.Errorf("whisper Usage() should contain <player>, got %q", got)
	}
}

// TestSlotAtFallsBackToArgs verifies that slotAt falls back to Args when a
// verb declares no SubArgs (whisper).
func TestSlotAtFallsBackToArgs(t *testing.T) {
	w, _ := SpecFor("whisper")
	got, ok := w.slotAt(1, []string{"whisper", "vm-a"})
	if !ok || got.kind != argPlayer {
		t.Fatalf("whisper slot 1 = %+v ok=%v, want argPlayer", got, ok)
	}
}

// TestSlotAtUsesSubArgs verifies that slotAt resolves a SubArgs-backed slot
// by the sub-verb token. A temporary fixture verb exercises the mechanism
// before real verbs adopt it.
func TestSlotAtUsesSubArgs(t *testing.T) {
	s := Spec{
		Verb: "fix",
		Args: []argSlot{{kind: argHost, name: "host"}, {kind: argFixed, options: []string{"a", "b"}, name: "sub"}},
		SubArgs: map[string][]argSlot{
			"a": {{kind: argPlayer, name: "p"}},
			"b": {{kind: argHost, name: "h2"}, {kind: argFree, name: "x"}},
		},
	}
	if got, ok := s.slotAt(2, []string{"fix", "vm-a", "a"}); !ok || got.kind != argPlayer {
		t.Fatalf("sub a slot 2 = %+v ok=%v, want argPlayer", got, ok)
	}
	if got, ok := s.slotAt(2, []string{"fix", "vm-a", "b"}); !ok || got.kind != argHost {
		t.Fatalf("sub b slot 2 = %+v ok=%v, want argHost", got, ok)
	}
	if got, ok := s.slotAt(3, []string{"fix", "vm-a", "b"}); !ok || got.kind != argFree {
		t.Fatalf("sub b slot 3 = %+v ok=%v, want argFree", got, ok)
	}
	if _, ok := s.slotAt(2, []string{"fix", "vm-a"}); ok {
		t.Fatalf("slot 2 with no sub typed should be absent")
	}
}

func TestPlayerVerbsHaveNameCompletion(t *testing.T) {
	// player: host, sub, name → name slot (idx 2) is a player slot.
	p, _ := SpecFor("player")
	if !p.IsPlayerPos(2, "player", "vm-a", "inspect") {
		t.Error("player name slot should be argPlayer")
	}
	// give: host, sub, name, rest → name slot (idx 2) is a player slot.
	g, _ := SpecFor("give")
	if !g.IsPlayerPos(2, "give", "vm-a", "skillpoints") {
		t.Error("give name slot should be argPlayer")
	}
	// admin already wired in Task 1 — sanity only.
	a, _ := SpecFor("admin")
	if !a.IsPlayerPos(2, "admin", "vm-a", "kick") {
		t.Error("admin player slot should be argPlayer")
	}
}

func TestGiveItemTemplateCompletes(t *testing.T) {
	g, _ := SpecFor("give")
	if !g.IsCatalogPos(3, "give", "vm-a", "item") {
		t.Error("give item template slot (idx 3) should be argCatalog")
	}
	if got := g.Candidates(3, nil, "give", "vm-a", "item"); len(got) == 0 {
		t.Error("give item template should offer the items catalog")
	}
	if !g.IsPlayerPos(2, "give", "vm-a", "skillpoints") {
		t.Error("give player slot (idx 2) should be argPlayer for every sub")
	}
	if g.IsCatalogPos(3, "give", "vm-a", "currency") {
		t.Error("give currency idx3 must NOT be a catalog slot")
	}
}

func TestSlotAtBoundaries(t *testing.T) {
	s := Spec{
		Verb: "demo",
		Args: []argSlot{{kind: argHost, name: "host"}, {kind: argFixed, options: []string{"a"}, name: "sub"}},
		SubArgs: map[string][]argSlot{
			"a": {{kind: argPlayer, name: "p"}},
		},
	}
	// pos == sub (the argFixed slot) resolves from Args even with a sub typed.
	if got, ok := s.slotAt(1, []string{"demo", "vm-a", "a"}); !ok || got.kind != argFixed {
		t.Errorf("pos==sub = %+v ok=%v, want argFixed from Args", got, ok)
	}
	// pos < sub (the host slot) resolves from Args.
	if got, ok := s.slotAt(0, []string{"demo", "vm-a", "a"}); !ok || got.kind != argHost {
		t.Errorf("pos<sub = %+v ok=%v, want argHost from Args", got, ok)
	}
	// sub-verb token not in SubArgs → fall through to Args (out of range here → absent).
	if _, ok := s.slotAt(2, []string{"demo", "vm-a", "zzz"}); ok {
		t.Errorf("unknown sub-verb should fall through to Args (absent at pos 2)")
	}
	// pos past the sub's SubArgs slots → absent.
	if _, ok := s.slotAt(3, []string{"demo", "vm-a", "a"}); ok {
		t.Errorf("pos beyond SubArgs slots should be absent")
	}
}
