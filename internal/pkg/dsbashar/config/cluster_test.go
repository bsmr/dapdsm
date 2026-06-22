package config

import (
	"testing"
)

func TestToData_FromData_RoundTrip(t *testing.T) {
	c := Config{Target: TargetProd, WorldName: "Arrakis", WorldRegion: "Europe",
		GamePortBase: 7777, AlwaysOnSets: []string{"Hagga"}, ServerDisplayName: "My Server", HostDatacenterID: "bg.example.test"}
	d := ToData(c, []byte("tok"), []byte("pw"))
	if d.Values["WORLD_NAME"] != "Arrakis" || d.Values["TARGET"] != "prod" {
		t.Fatalf("values = %v", d.Values)
	}
	if string(d.Secrets["fls-token"]) != "tok" || string(d.Secrets["server-password"]) != "pw" {
		t.Fatalf("secrets = %v", d.Secrets)
	}
	back := FromData(d)
	if back.WorldName != "Arrakis" || back.WorldRegion != "Europe" || back.GamePortBase != 7777 {
		t.Fatalf("round-trip lost data: %+v", back)
	}
	if len(back.AlwaysOnSets) != 1 || back.AlwaysOnSets[0] != "Hagga" {
		t.Fatalf("AlwaysOnSets lost: %v", back.AlwaysOnSets)
	}
	if back.HostDatacenterID != c.HostDatacenterID {
		t.Errorf("HostDatacenterID = %q, want %q", back.HostDatacenterID, c.HostDatacenterID)
	}
}

func TestMerge_OverrideWins(t *testing.T) {
	base := Config{WorldName: "Old", WorldRegion: "Europe"}
	got := Merge(base, Override{WorldName: "New"})
	if got.WorldName != "New" || got.WorldRegion != "Europe" {
		t.Fatalf("merge = %+v", got)
	}
}

func TestMerge_AppliesHostDatacenterID(t *testing.T) {
	base := Config{HostDatacenterID: "dune-testing"}
	got := Merge(base, Override{HostDatacenterID: "bg.example.test"})
	if got.HostDatacenterID != "bg.example.test" {
		t.Fatalf("HostDatacenterID = %q, want bg.example.test", got.HostDatacenterID)
	}
	// empty override leaves the base untouched
	got = Merge(base, Override{})
	if got.HostDatacenterID != "dune-testing" {
		t.Fatalf("empty override changed HostDatacenterID to %q", got.HostDatacenterID)
	}
}
