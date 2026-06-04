package command

import (
	"strings"
	"testing"
)

// TestCatalogCandidates_Items verifies that catalogCandidates("items") returns
// item ids from the vendored catalog.
func TestCatalogCandidates_Items(t *testing.T) {
	cands := catalogCandidates("items")
	if len(cands) == 0 {
		t.Fatal("catalogCandidates(items): got empty slice")
	}
	// T6_Augment_Acuracy1 is the first item in the vendored catalog.
	found := false
	for _, c := range cands {
		if c == "T6_Augment_Acuracy1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("item catalog does not contain T6_Augment_Acuracy1 (first 5: %v)", cands[:min5(cands)])
	}
}

// TestCatalogCandidates_Skills verifies that catalogCandidates("skills") returns
// skill module ids.
func TestCatalogCandidates_Skills(t *testing.T) {
	cands := catalogCandidates("skills")
	if len(cands) == 0 {
		t.Fatal("catalogCandidates(skills): got empty slice")
	}
	// Skills.Ability.Hypersprint is in the vendored catalog.
	found := false
	for _, c := range cands {
		if c == "Skills.Ability.Hypersprint" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("skill catalog does not contain Skills.Ability.Hypersprint (first 5: %v)", cands[:min5(cands)])
	}
}

// TestCatalogCandidates_Vehicles verifies that catalogCandidates("vehicles")
// returns vehicle ids.
func TestCatalogCandidates_Vehicles(t *testing.T) {
	cands := catalogCandidates("vehicles")
	if len(cands) == 0 {
		t.Fatal("catalogCandidates(vehicles): got empty slice")
	}
	found := false
	for _, c := range cands {
		if c == "Sandbike" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("vehicle catalog does not contain Sandbike (got: %v)", cands)
	}
}

// TestCatalogCandidates_Unknown verifies that an unknown key defaults to items.
func TestCatalogCandidates_Unknown(t *testing.T) {
	cands := catalogCandidates("unknown-key")
	if len(cands) == 0 {
		t.Fatal("catalogCandidates(unknown): expected items fallback, got empty slice")
	}
}

// TestAdminSpec_ArgCatalog_ItemsViaSubArgs verifies that the admin name-arg slot
// at position 3 returns the items catalog when the sub-verb is "item".
func TestAdminSpec_ArgCatalog_ItemsViaSubArgs(t *testing.T) {
	s, ok := SpecFor("admin")
	if !ok {
		t.Fatal("SpecFor(admin) not found")
	}
	// SubArgs["item"] = [{argPlayer, "player-id"}, {argCatalog, "items", "name"}]
	// slotAt(3, tokens) resolves relative to the sub-verb slot (1), so pos 3
	// maps to SubArgs["item"][1] = items catalog.
	cands := s.Candidates(3, nil, "admin", "vm-a", "item")
	if len(cands) == 0 {
		t.Fatal("Candidates for admin item name slot: got empty slice")
	}
}

// TestCandidatesBackwardCompat verifies that existing 2-arg calls still work.
func TestCandidatesBackwardCompat(t *testing.T) {
	s, _ := SpecFor("lifecycle")
	// existing call pattern: Candidates(pos, hosts)
	got := s.Candidates(0, []string{"vm-x"})
	if len(got) != 1 || got[0] != "vm-x" {
		t.Errorf("backward-compat Candidates(0, hosts) = %v, want [vm-x]", got)
	}
}

func min5(s []string) int {
	if len(s) < 5 {
		return len(s)
	}
	return 5
}

// TestAdminSubArgsCatalogSelection proves that the admin catalog resolves via
// SubArgs: skill/vehicle expose catalog slots, kick exposes none.
func TestAdminSubArgsCatalogSelection(t *testing.T) {
	a, _ := SpecFor("admin")
	if got := a.Candidates(3, nil, "admin", "vm-a", "skill"); len(got) == 0 {
		t.Error("admin skill should expose the skills catalog at slot 3")
	}
	if got := a.Candidates(3, nil, "admin", "vm-a", "vehicle"); len(got) == 0 {
		t.Error("admin vehicle should expose the vehicles catalog at slot 3")
	}
	if !a.IsPlayerPos(2, "admin", "vm-a", "kick") {
		t.Error("admin player-id slot (idx 2) should be argPlayer for every verb")
	}
	if a.IsCatalogPos(3, "admin", "vm-a", "kick") {
		t.Error("admin kick must not expose a catalog slot")
	}
}

// TestSuggestAdminCatalogFiltered verifies prefix-filtering of catalog ids.
// Tests catalogCandidates in isolation; end-to-end coverage is in TestAdminSubArgsCatalogSelection.
func TestSuggestAdminCatalog_PrefixFilter(t *testing.T) {
	cands := catalogCandidates("items")
	// Filter by prefix "T6_Augment_Ac" — should match T6_Augment_Acuracy1
	prefix := "T6_Augment_Ac"
	var filtered []string
	for _, c := range cands {
		if strings.HasPrefix(c, prefix) {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		t.Errorf("prefix %q matched nothing from %d candidates", prefix, len(cands))
	}
}
