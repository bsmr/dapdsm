package command

import (
	"strings"
	"testing"

	admincatalog "go.muehmer.eu/dapdsm/internal/pkg/dunemgr/admin/catalog"
	"go.muehmer.eu/dapdsm/pkg/domain/dbquery"
)

func TestFormatInspectIsTabular(t *testing.T) {
	d := &dbquery.PlayerDetail{
		FLSID: "FLS1", Found: true, CharacterName: "Stilgar", OnlineStatus: "Offline",
		Progression: &dbquery.Progression{TotalSkillPoints: 42, UnspentSkillPoints: 7},
		Vitals:      &dbquery.Vitals{CurrentHealth: 150},
	}
	out := FormatInspect(d)
	for _, want := range []string{"Stilgar", "skill points", "42", "health", "150"} {
		if !strings.Contains(out, want) {
			t.Errorf("FormatInspect missing %q:\n%s", want, out)
		}
	}
}

func TestFormatInspectNotFound(t *testing.T) {
	out := FormatInspect(&dbquery.PlayerDetail{FLSID: "X"})
	if !strings.Contains(out, "no player") {
		t.Fatalf("expected not-found line: %q", out)
	}
}

func TestFormatInspectNumbersInventories(t *testing.T) {
	d := &dbquery.PlayerDetail{
		Found: true, CharacterName: "Mal", FLSID: "DEADBEEF",
		Inventories: []dbquery.InvBreakdown{{InventoryType: 3, ItemCount: 42}, {InventoryType: 7, ItemCount: 8}},
	}
	out := FormatInspect(d)
	if !strings.Contains(out, "[1] type 3") || !strings.Contains(out, "[2] type 7") {
		t.Fatalf("inventories not numbered:\n%s", out)
	}
}

func TestFormatInventoryItems(t *testing.T) {
	rows := []dbquery.ItemRow{{ID: 8841, TemplateID: "Spice", StackSize: 120, Quality: 5}}
	out := FormatInventoryItems(3, rows)
	if !strings.Contains(out, "[1]") || !strings.Contains(out, "id=8841") || !strings.Contains(out, "Spice") {
		t.Fatalf("item listing wrong:\n%s", out)
	}
}

func TestFormatInventoryItemsUsesDisplayName(t *testing.T) {
	rows := []dbquery.ItemRow{{ID: 1, TemplateID: "Ammo", StackSize: 5, Quality: 0}}
	out := FormatInventoryItems(0, rows)
	if !strings.Contains(out, admincatalog.DisplayName("Ammo")) {
		t.Fatalf("display name not used:\n%s", out)
	}
}

func TestInventoryTypeName(t *testing.T) {
	cases := map[int]string{0: "Backpack", 1: "Equipment", 14: "Emotes", 15: "Hotbar", 27: "Emotes (2)", 29: "Contracts"}
	for typ, want := range cases {
		if got := InventoryTypeName(typ); got != want {
			t.Errorf("InventoryTypeName(%d)=%q want %q", typ, got, want)
		}
	}
	if got := InventoryTypeName(99); got != "type 99" {
		t.Errorf("unknown type = %q want %q", got, "type 99")
	}
}
