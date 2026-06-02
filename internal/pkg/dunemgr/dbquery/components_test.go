package dbquery

import "testing"

const fixtureComponents = `{
  "FLevelComponent":[0,{"TotalSkillPoints":42,"UnspentSkillPoints":7,"TotalXPEarned":12345,
    "KeystoneBonusSkillPoints":3,"RespecCostCounter":1,
    "ActivePerkTags":["a","b"],"ActiveAbilityTags":["x"]}],
  "FHealthComponent":[0,{"m_CurrentHealth":150.0,"m_CurrentDownButNotOutStateHealth":100.0,
    "m_MaxDownButNotOutStateHealth":100.0}],
  "FSpiceAddictionComponent":[0,{"SystemStatus":"AddictionDisabled","SpiceVisionEnabledStatus":"FullyEnabled"}]
}`

func TestParseProgression(t *testing.T) {
	p, ok := parseProgression([]byte(fixtureComponents))
	if !ok {
		t.Fatal("want ok")
	}
	if p.TotalSkillPoints != 42 || p.UnspentSkillPoints != 7 || p.TotalXPEarned != 12345 {
		t.Fatalf("scalars wrong: %+v", p)
	}
	if p.KeystoneBonusSkillPoints != 3 || p.RespecCostCounter != 1 {
		t.Fatalf("scalars2 wrong: %+v", p)
	}
	if p.ActivePerks != 2 || p.ActiveAbilities != 1 {
		t.Fatalf("tag counts wrong: %+v", p)
	}
}

func TestParseVitals(t *testing.T) {
	v, ok := parseVitals([]byte(fixtureComponents))
	if !ok {
		t.Fatal("want ok")
	}
	if v.CurrentHealth != 150.0 {
		t.Fatalf("CurrentHealth want 150.0 got %v", v.CurrentHealth)
	}
	if v.DownCurrent != 100.0 {
		t.Fatalf("DownCurrent want 100.0 got %v", v.DownCurrent)
	}
	if v.DownMax != 100.0 {
		t.Fatalf("DownMax want 100.0 got %v", v.DownMax)
	}
}

func TestParseSpice(t *testing.T) {
	s, ok := parseSpice([]byte(fixtureComponents))
	if !ok || s.SystemStatus != "AddictionDisabled" || s.SpiceVision != "FullyEnabled" {
		t.Fatalf("spice wrong: %+v ok=%v", s, ok)
	}
}

func TestParseMissingComponentReturnsFalse(t *testing.T) {
	if _, ok := parseProgression([]byte(`{}`)); ok {
		t.Fatal("missing FLevelComponent should return ok=false")
	}
	if _, ok := parseVitals([]byte(`not json`)); ok {
		t.Fatal("invalid json should return ok=false")
	}
	if _, ok := parseSpice([]byte(`{}`)); ok {
		t.Fatal("missing FSpiceAddictionComponent should return ok=false")
	}
}
