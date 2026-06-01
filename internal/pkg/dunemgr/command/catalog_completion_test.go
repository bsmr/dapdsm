package command

import (
	"strings"
	"testing"
)

// TestAdminSpec_ArgCatalog_ItemVerb verifies that the admin name-arg slot for
// verb "item" returns item ids from the catalog.
func TestAdminSpec_ArgCatalog_ItemVerb(t *testing.T) {
	s, ok := SpecFor("admin")
	if !ok {
		t.Fatal("SpecFor(admin) not found")
	}
	// admin arg layout: host(0) verb(1) player(2) name(3)
	// tokens is the full prefix-token list: [outerVerb, host, adminSubVerb, playerID]
	cands := s.Candidates(3, nil, "admin", "vm-a", "item", "player-x")
	if len(cands) == 0 {
		t.Fatal("Candidates for admin name slot with verb=item: got empty slice")
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
		t.Errorf("item catalog candidates do not contain T6_Augment_Acuracy1 (first 5: %v)", cands[:min5(cands)])
	}
}

// TestAdminSpec_ArgCatalog_SkillVerb verifies that verb "skill" returns skill
// module ids.
func TestAdminSpec_ArgCatalog_SkillVerb(t *testing.T) {
	s, _ := SpecFor("admin")
	cands := s.Candidates(3, nil, "admin", "vm-a", "skill", "player-x")
	if len(cands) == 0 {
		t.Fatal("Candidates for admin name slot with verb=skill: got empty slice")
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
		t.Errorf("skill catalog candidates do not contain Skills.Ability.Hypersprint (first 5: %v)", cands[:min5(cands)])
	}
}

// TestAdminSpec_ArgCatalog_VehicleVerb verifies that verb "vehicle" returns
// vehicle ids.
func TestAdminSpec_ArgCatalog_VehicleVerb(t *testing.T) {
	s, _ := SpecFor("admin")
	cands := s.Candidates(3, nil, "admin", "vm-a", "vehicle", "player-x")
	if len(cands) == 0 {
		t.Fatal("Candidates for admin name slot with verb=vehicle: got empty slice")
	}
	found := false
	for _, c := range cands {
		if c == "Sandbike" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("vehicle catalog candidates do not contain Sandbike (got: %v)", cands)
	}
}

// TestAdminSpec_ArgCatalog_NoTokens verifies that without typed tokens the
// catalog slot still returns the default (items) catalog.
func TestAdminSpec_ArgCatalog_NoTokens(t *testing.T) {
	s, _ := SpecFor("admin")
	cands := s.Candidates(3, nil) // no tokens — default to items
	if len(cands) == 0 {
		t.Fatal("Candidates for admin name slot with no tokens: got empty slice")
	}
}

// TestAdminSpec_ArgCatalog_UnknownVerb verifies that a non-catalog verb still
// returns a non-nil (defaulting to items) list from the name slot.
func TestAdminSpec_ArgCatalog_UnknownVerb(t *testing.T) {
	s, _ := SpecFor("admin")
	// "kick" has no catalog name; default to items.
	cands := s.Candidates(3, nil, "admin", "vm-a", "kick", "player-x")
	// kick has no name arg in practice, but the slot is defined as argCatalog;
	// it should return items (the default catalog key) not nil.
	if cands == nil {
		t.Error("expected non-nil candidates for argCatalog slot, even for kick")
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

// TestSuggestAdminCatalogFiltered verifies that the suggest function in the
// tui package filters catalog ids by prefix when an admin command is typed.
// This test is in the command package; the tui-side test is in tui/complete_test.go.
func TestSuggestAdminCatalog_PrefixFilter(t *testing.T) {
	s, _ := SpecFor("admin")
	// tokens is the full prefix-token list: [outerVerb, host, adminSubVerb, playerID]
	cands := s.Candidates(3, nil, "admin", "vm-a", "item", "player-x")
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
