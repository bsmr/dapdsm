package admin

import (
	"strings"
	"testing"
)

// TestBuild_CatalogValidation_Item_KnownID verifies that a known item id builds
// successfully. "T6_Augment_Acuracy1" is the first entry in the vendored catalog.
func TestBuild_CatalogValidation_Item_KnownID(t *testing.T) {
	_, err := Build("item", "player-1", map[string]string{
		"ItemName": "T6_Augment_Acuracy1",
	})
	if err != nil {
		t.Fatalf("Build item with known id: %v", err)
	}
}

// TestBuild_CatalogValidation_Item_UnknownID verifies that an unknown item id
// is rejected with a descriptive error.
func TestBuild_CatalogValidation_Item_UnknownID(t *testing.T) {
	_, err := Build("item", "player-1", map[string]string{
		"ItemName": "BP_Item_Resource_Spice_C",
	})
	if err == nil {
		t.Fatal("Build item with unknown id: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown item") {
		t.Errorf("error should mention 'unknown item', got: %v", err)
	}
}

// TestBuild_CatalogValidation_Skill_KnownID verifies that a known skill module
// id builds successfully. "Skills.Ability.Hypersprint" has maxLevel=3.
func TestBuild_CatalogValidation_Skill_KnownID(t *testing.T) {
	_, err := Build("skill", "player-2", map[string]string{
		"Module": "Skills.Ability.Hypersprint",
		"Level":  "2",
	})
	if err != nil {
		t.Fatalf("Build skill with known id: %v", err)
	}
}

// TestBuild_CatalogValidation_Skill_UnknownID verifies that an unknown module
// is rejected.
func TestBuild_CatalogValidation_Skill_UnknownID(t *testing.T) {
	_, err := Build("skill", "player-2", map[string]string{
		"Module": "Skills.NoSuch.Module",
		"Level":  "1",
	})
	if err == nil {
		t.Fatal("Build skill with unknown id: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown skill module") {
		t.Errorf("error should mention 'unknown skill module', got: %v", err)
	}
}

// TestBuild_CatalogValidation_Skill_LevelExceedsMax verifies that Level >
// maxLevel is rejected. "Skills.Ability.Hypersprint" has maxLevel=3.
func TestBuild_CatalogValidation_Skill_LevelExceedsMax(t *testing.T) {
	_, err := Build("skill", "player-2", map[string]string{
		"Module": "Skills.Ability.Hypersprint",
		"Level":  "99",
	})
	if err == nil {
		t.Fatal("Build skill with level > maxLevel: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "max") {
		t.Errorf("error should mention max level, got: %v", err)
	}
}

// TestBuild_CatalogValidation_Vehicle_KnownID verifies that a known vehicle
// class with a valid template builds successfully.
func TestBuild_CatalogValidation_Vehicle_KnownID(t *testing.T) {
	_, err := Build("vehicle", "player-3", map[string]string{
		"ClassName":    "Sandbike",
		"TemplateName": "T1_ExtraSeat",
		"X":            "0",
		"Y":            "0",
		"Z":            "0",
	})
	if err != nil {
		t.Fatalf("Build vehicle with known id+template: %v", err)
	}
}

// TestBuild_CatalogValidation_Vehicle_UnknownClassName verifies that an
// unknown vehicle class name is rejected.
func TestBuild_CatalogValidation_Vehicle_UnknownClassName(t *testing.T) {
	_, err := Build("vehicle", "player-3", map[string]string{
		"ClassName":    "BP_Vehicle_SandCrawler_C",
		"TemplateName": "Default",
		"X":            "0",
		"Y":            "0",
		"Z":            "0",
	})
	if err == nil {
		t.Fatal("Build vehicle with unknown class: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown vehicle") {
		t.Errorf("error should mention 'unknown vehicle', got: %v", err)
	}
}

// TestBuild_CatalogValidation_Vehicle_BadTemplateName verifies that a
// TemplateName not in the vehicle's template list is rejected.
func TestBuild_CatalogValidation_Vehicle_BadTemplateName(t *testing.T) {
	_, err := Build("vehicle", "player-3", map[string]string{
		"ClassName":    "Sandbike",
		"TemplateName": "Default",
		"X":            "0",
		"Y":            "0",
		"Z":            "0",
	})
	if err == nil {
		t.Fatal("Build vehicle with bad template: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown template") {
		t.Errorf("error should mention 'unknown template', got: %v", err)
	}
}
