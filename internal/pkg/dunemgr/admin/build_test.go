package admin

import (
	"encoding/json"
	"strings"
	"testing"
)

// unmarshalPayload decodes a JSON string produced by Build into a
// map[string]any for field-level assertions.
func unmarshalPayload(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("unmarshal: %v (raw: %q)", err, s)
	}
	return m
}

// --- item ---

func TestBuild_Item_Happy(t *testing.T) {
	// T6_Augment_Acuracy1 is a real item id in the vendored catalog.
	got, err := Build("item", "player-123", map[string]string{
		"ItemName":   "T6_Augment_Acuracy1",
		"Quantity":   "5",
		"Durability": "0.9",
	})
	if err != nil {
		t.Fatalf("Build item: %v", err)
	}
	m := unmarshalPayload(t, got)
	if m["ServerCommand"] != "AddItemToInventory" {
		t.Errorf("ServerCommand=%q", m["ServerCommand"])
	}
	if m["PlayerId"] != "player-123" {
		t.Errorf("PlayerId=%q", m["PlayerId"])
	}
	if m["ItemName"] != "T6_Augment_Acuracy1" {
		t.Errorf("ItemName=%q", m["ItemName"])
	}
	if qty, ok := m["Quantity"].(float64); !ok || qty != 5 {
		t.Errorf("Quantity=%v", m["Quantity"])
	}
	if dur, ok := m["Durability"].(float64); !ok || dur != 0.9 {
		t.Errorf("Durability=%v", m["Durability"])
	}
}

func TestBuild_Item_Defaults(t *testing.T) {
	// T6_Augment_Acuracy1 is a real item id in the vendored catalog.
	got, err := Build("item", "player-123", map[string]string{
		"ItemName": "T6_Augment_Acuracy1",
	})
	if err != nil {
		t.Fatalf("Build item defaults: %v", err)
	}
	m := unmarshalPayload(t, got)
	// Default Quantity=1, Durability=1.0
	if qty, ok := m["Quantity"].(float64); !ok || qty != 1 {
		t.Errorf("Quantity default: %v", m["Quantity"])
	}
	if dur, ok := m["Durability"].(float64); !ok || dur != 1.0 {
		t.Errorf("Durability default: %v", m["Durability"])
	}
}

func TestBuild_Item_MissingItemName(t *testing.T) {
	_, err := Build("item", "player-123", map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing ItemName, got nil")
	}
	if !strings.Contains(err.Error(), "ItemName") {
		t.Errorf("error does not mention ItemName: %v", err)
	}
}

// --- water ---

func TestBuild_Water_Default(t *testing.T) {
	got, err := Build("water", "player-abc", map[string]string{})
	if err != nil {
		t.Fatalf("Build water: %v", err)
	}
	m := unmarshalPayload(t, got)
	if m["ServerCommand"] != "UpdateAllWaterFillables" {
		t.Errorf("ServerCommand=%q", m["ServerCommand"])
	}
	if amt, ok := m["WaterAmount"].(float64); !ok || amt != 1000000 {
		t.Errorf("WaterAmount default: %v", m["WaterAmount"])
	}
}

func TestBuild_Water_Override(t *testing.T) {
	got, err := Build("water", "p1", map[string]string{"WaterAmount": "5000"})
	if err != nil {
		t.Fatalf("Build water override: %v", err)
	}
	m := unmarshalPayload(t, got)
	if amt, ok := m["WaterAmount"].(float64); !ok || amt != 5000 {
		t.Errorf("WaterAmount=%v, want 5000", m["WaterAmount"])
	}
}

// --- xp ---

func TestBuild_XP_InjectsCategory(t *testing.T) {
	got, err := Build("xp", "player-x", map[string]string{})
	if err != nil {
		t.Fatalf("Build xp: %v", err)
	}
	m := unmarshalPayload(t, got)
	if m["ServerCommand"] != "AwardXP" {
		t.Errorf("ServerCommand=%q", m["ServerCommand"])
	}
	if m["Category"] != "Combat" {
		t.Errorf("Category=%q, want Combat", m["Category"])
	}
	if exp, ok := m["Experience"].(float64); !ok || exp != 1000 {
		t.Errorf("Experience default: %v", m["Experience"])
	}
}

// --- skill ---

func TestBuild_Skill_Happy(t *testing.T) {
	// Skills.Ability.Hypersprint is a real skill id with maxLevel=3 in the vendored catalog.
	got, err := Build("skill", "p2", map[string]string{
		"Module": "Skills.Ability.Hypersprint",
		"Level":  "3",
	})
	if err != nil {
		t.Fatalf("Build skill: %v", err)
	}
	m := unmarshalPayload(t, got)
	if m["ServerCommand"] != "SkillsSetModuleLevel" {
		t.Errorf("ServerCommand=%q", m["ServerCommand"])
	}
	if m["Module"] != "Skills.Ability.Hypersprint" {
		t.Errorf("Module=%q", m["Module"])
	}
	if lvl, ok := m["Level"].(float64); !ok || lvl != 3 {
		t.Errorf("Level=%v", m["Level"])
	}
}

func TestBuild_Skill_MissingModule(t *testing.T) {
	_, err := Build("skill", "p2", map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing Module")
	}
	if !strings.Contains(err.Error(), "Module") {
		t.Errorf("error does not mention Module: %v", err)
	}
}

// --- skillpoints ---

func TestBuild_SkillPoints_Default(t *testing.T) {
	got, err := Build("skillpoints", "p3", map[string]string{})
	if err != nil {
		t.Fatalf("Build skillpoints: %v", err)
	}
	m := unmarshalPayload(t, got)
	if m["ServerCommand"] != "SkillsSetUnspentSkillPoints" {
		t.Errorf("ServerCommand=%q", m["ServerCommand"])
	}
	if pts, ok := m["SkillPoints"].(float64); !ok || pts != 0 {
		t.Errorf("SkillPoints default: %v", m["SkillPoints"])
	}
}

// --- vehicle ---

func TestBuild_Vehicle_Happy(t *testing.T) {
	// Sandbike/T1_ExtraSeat is a real vehicle id+template in the vendored catalog.
	got, err := Build("vehicle", "p4", map[string]string{
		"ClassName":    "Sandbike",
		"TemplateName": "T1_ExtraSeat",
		"X":            "1000.5",
		"Y":            "2000.0",
		"Z":            "50.0",
	})
	if err != nil {
		t.Fatalf("Build vehicle: %v", err)
	}
	m := unmarshalPayload(t, got)
	if m["ServerCommand"] != "SpawnVehicleAt" {
		t.Errorf("ServerCommand=%q", m["ServerCommand"])
	}
	if m["ClassName"] != "Sandbike" {
		t.Errorf("ClassName=%q", m["ClassName"])
	}
	if m["TemplateName"] != "T1_ExtraSeat" {
		t.Errorf("TemplateName=%q", m["TemplateName"])
	}
	if x, ok := m["X"].(float64); !ok || x != 1000.5 {
		t.Errorf("X=%v", m["X"])
	}
	// Persistent default=1.0 must be present.
	if p, ok := m["Persistent"].(float64); !ok || p != 1.0 {
		t.Errorf("Persistent default: %v", m["Persistent"])
	}
}

func TestBuild_Vehicle_MissingRequired(t *testing.T) {
	// Missing TemplateName, X, Y, Z. Sandcrawler is a real vehicle id.
	_, err := Build("vehicle", "p4", map[string]string{
		"ClassName": "Sandcrawler",
	})
	if err == nil {
		t.Fatal("expected error for missing required fields")
	}
}

func TestBuild_Vehicle_OptionalAbsent(t *testing.T) {
	// Rotation, Faction are optional with no default → must NOT appear in payload.
	// Sandbike/T1_ExtraSeat is a real vehicle id+template.
	got, err := Build("vehicle", "p4", map[string]string{
		"ClassName":    "Sandbike",
		"TemplateName": "T1_ExtraSeat",
		"X":            "0",
		"Y":            "0",
		"Z":            "0",
	})
	if err != nil {
		t.Fatalf("Build vehicle optional: %v", err)
	}
	m := unmarshalPayload(t, got)
	if _, present := m["Rotation"]; present {
		t.Errorf("Rotation must be absent when not provided, got: %v", m["Rotation"])
	}
	if _, present := m["Faction"]; present {
		t.Errorf("Faction must be absent when not provided, got: %v", m["Faction"])
	}
}

// --- teleport ---

func TestBuild_Teleport_Happy(t *testing.T) {
	got, err := Build("teleport", "p5", map[string]string{
		"X": "100.0",
		"Y": "200.0",
		"Z": "300.0",
	})
	if err != nil {
		t.Fatalf("Build teleport: %v", err)
	}
	m := unmarshalPayload(t, got)
	if m["ServerCommand"] != "TeleportTo" {
		t.Errorf("ServerCommand=%q", m["ServerCommand"])
	}
}

func TestBuild_TeleportExact_Happy(t *testing.T) {
	got, err := Build("teleport-exact", "p6", map[string]string{
		"X": "10",
		"Y": "20",
		"Z": "30",
	})
	if err != nil {
		t.Fatalf("Build teleport-exact: %v", err)
	}
	m := unmarshalPayload(t, got)
	if m["ServerCommand"] != "TeleportToExact" {
		t.Errorf("ServerCommand=%q", m["ServerCommand"])
	}
}

func TestBuild_Teleport_OptionalAbsent(t *testing.T) {
	// Yaw, CamPitch, CamYaw, CamRoll are optional; must be absent.
	got, err := Build("teleport", "p5", map[string]string{
		"X": "0", "Y": "0", "Z": "0",
	})
	if err != nil {
		t.Fatalf("Build teleport optional absent: %v", err)
	}
	m := unmarshalPayload(t, got)
	for _, k := range []string{"Yaw", "CamPitch", "CamYaw", "CamRoll"} {
		if _, present := m[k]; present {
			t.Errorf("%s must be absent when not provided, got: %v", k, m[k])
		}
	}
}

// --- destructive verbs ---

func TestBuild_Kick_OnlyBaseFields(t *testing.T) {
	got, err := Build("kick", "player-kick", map[string]string{})
	if err != nil {
		t.Fatalf("Build kick: %v", err)
	}
	m := unmarshalPayload(t, got)
	if m["ServerCommand"] != "KickPlayer" {
		t.Errorf("ServerCommand=%q", m["ServerCommand"])
	}
	if m["PlayerId"] != "player-kick" {
		t.Errorf("PlayerId=%q", m["PlayerId"])
	}
	if len(m) != 2 {
		t.Errorf("unexpected extra fields in kick payload: %v", m)
	}
}

func TestBuild_Clean_OnlyBaseFields(t *testing.T) {
	got, err := Build("clean", "player-clean", map[string]string{})
	if err != nil {
		t.Fatalf("Build clean: %v", err)
	}
	m := unmarshalPayload(t, got)
	if m["ServerCommand"] != "CleanPlayerInventory" {
		t.Errorf("ServerCommand=%q", m["ServerCommand"])
	}
	if len(m) != 2 {
		t.Errorf("extra fields in clean payload: %v", m)
	}
}

func TestBuild_Reset_OnlyBaseFields(t *testing.T) {
	got, err := Build("reset", "player-reset", map[string]string{})
	if err != nil {
		t.Fatalf("Build reset: %v", err)
	}
	m := unmarshalPayload(t, got)
	if m["ServerCommand"] != "ResetProgression" {
		t.Errorf("ServerCommand=%q", m["ServerCommand"])
	}
	if len(m) != 2 {
		t.Errorf("extra fields in reset payload: %v", m)
	}
}

// --- unknown verb ---

func TestBuild_UnknownVerb(t *testing.T) {
	_, err := Build("notaverb", "p", map[string]string{})
	if err == nil {
		t.Fatal("expected error for unknown verb")
	}
	if !strings.Contains(err.Error(), "unknown verb") {
		t.Errorf("error missing 'unknown verb': %v", err)
	}
}

// --- Verbs ---

func TestVerbs_Sorted(t *testing.T) {
	v := Verbs()
	if len(v) == 0 {
		t.Fatal("Verbs() returned empty slice")
	}
	for i := 1; i < len(v); i++ {
		if v[i] < v[i-1] {
			t.Errorf("Verbs() not sorted at [%d]: %q < %q", i, v[i], v[i-1])
		}
	}
}

func TestVerbs_ContainsAll(t *testing.T) {
	v := Verbs()
	want := []string{
		"clean", "item", "kick", "reset", "skill", "skillpoints",
		"teleport", "teleport-exact", "vehicle", "water", "xp",
	}
	if len(v) != len(want) {
		t.Errorf("Verbs() len=%d, want %d: %v", len(v), len(want), v)
	}
}
