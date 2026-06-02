package command

import (
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/dbquery"
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
