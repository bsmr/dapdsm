package catalog

import "testing"

// TestItemCatalog_NonEmpty checks that the embedded items.json parsed correctly.
func TestItemCatalog_NonEmpty(t *testing.T) {
	items := Items()
	if len(items) == 0 {
		t.Fatal("Items() returned empty slice")
	}
}

// TestItemIDs_ContainsKnown checks that a known item id is present.
// "T6_Augment_Acuracy1" is the first entry in the upstream items.json.
func TestItemIDs_ContainsKnown(t *testing.T) {
	ids := ItemIDs()
	if len(ids) == 0 {
		t.Fatal("ItemIDs() returned empty slice")
	}
	const want = "T6_Augment_Acuracy1"
	for _, id := range ids {
		if id == want {
			return
		}
	}
	t.Errorf("ItemIDs() does not contain %q", want)
}

// TestSkillCatalog_NonEmpty checks that the embedded skill-modules.json parsed correctly.
func TestSkillCatalog_NonEmpty(t *testing.T) {
	if len(Skills()) == 0 {
		t.Fatal("Skills() returned empty slice")
	}
}

// TestSkillMaxLevel_KnownID checks a skill with maxLevel > 1.
// "Skills.Ability.Hypersprint" has maxLevel=3 in the upstream catalog.
func TestSkillMaxLevel_KnownID(t *testing.T) {
	const id = "Skills.Ability.Hypersprint"
	max, ok := SkillMaxLevel(id)
	if !ok {
		t.Fatalf("SkillMaxLevel(%q): not found", id)
	}
	if max != 3 {
		t.Errorf("SkillMaxLevel(%q) = %d, want 3", id, max)
	}
}

// TestSkillMaxLevel_UnknownID checks that an unknown id returns ok=false.
func TestSkillMaxLevel_UnknownID(t *testing.T) {
	_, ok := SkillMaxLevel("Skills.NoSuch.Module")
	if ok {
		t.Error("SkillMaxLevel for unknown id should return ok=false")
	}
}

// TestVehicleCatalog_NonEmpty checks that the embedded vehicles.json parsed correctly.
func TestVehicleCatalog_NonEmpty(t *testing.T) {
	if len(Vehicles()) == 0 {
		t.Fatal("Vehicles() returned empty slice")
	}
}

// TestVehicleTemplates_KnownID checks that Sandbike has its expected templates.
func TestVehicleTemplates_KnownID(t *testing.T) {
	templates, ok := VehicleTemplates("Sandbike")
	if !ok {
		t.Fatal("VehicleTemplates(\"Sandbike\"): not found")
	}
	if len(templates) == 0 {
		t.Fatal("VehicleTemplates(\"Sandbike\"): empty templates")
	}
	// "T1_ExtraSeat" is the first template for Sandbike in the upstream catalog.
	const want = "T1_ExtraSeat"
	for _, tmpl := range templates {
		if tmpl == want {
			return
		}
	}
	t.Errorf("VehicleTemplates(\"Sandbike\") does not contain %q, got %v", want, templates)
}

// TestVehicleTemplates_UnknownID checks that an unknown id returns ok=false.
func TestVehicleTemplates_UnknownID(t *testing.T) {
	_, ok := VehicleTemplates("NoSuchVehicle")
	if ok {
		t.Error("VehicleTemplates for unknown id should return ok=false")
	}
}

// TestDisplayName verifies that DisplayName returns the human name for a known
// id (Ammo → "Light Darts") and falls back to the id for unknown entries.
func TestDisplayName(t *testing.T) {
	// "Ammo" is in items.json with name "Light Darts" — differs from the id.
	const known = "Ammo"
	if got := DisplayName(known); got == "" || got == known {
		t.Fatalf("DisplayName(%q)=%q want a non-id display name", known, got)
	}
	if got := DisplayName("NoSuchTemplate_X"); got != "NoSuchTemplate_X" {
		t.Fatalf("DisplayName(unknown)=%q want the id unchanged", got)
	}
}

// TestCatalogCounts verifies the embedded catalogs have the expected entry counts.
func TestCatalogCounts(t *testing.T) {
	if got := len(Items()); got < 2000 {
		t.Errorf("Items() count = %d, want >= 2000", got)
	}
	if got := len(Skills()); got < 100 {
		t.Errorf("Skills() count = %d, want >= 100", got)
	}
	if got := len(Vehicles()); got < 5 {
		t.Errorf("Vehicles() count = %d, want >= 5", got)
	}
}

// TestStackMax verifies StackMax returns the correct maximum stack size.
// "Radiation_Suit" is non-stackable (stack_max=1) in the upstream catalog.
// "AluminiumBar" is stackable (stack_max=500) in the upstream catalog.
func TestStackMax(t *testing.T) {
	if got := StackMax("Radiation_Suit"); got != 1 {
		t.Fatalf("StackMax(Radiation_Suit)=%d want 1 (non-stackable)", got)
	}
	if got := StackMax("AluminiumBar"); got != 500 {
		t.Fatalf("StackMax(AluminiumBar)=%d want 500 (stackable)", got)
	}
	if StackMax("NoSuchTemplate_X") != 0 {
		t.Fatalf("unknown template must return 0")
	}
}
