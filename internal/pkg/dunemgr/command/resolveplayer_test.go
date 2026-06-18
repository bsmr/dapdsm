package command

import (
	"bytes"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/domain/gamedb"
)

func TestFormatAmbiguousListsCandidates(t *testing.T) {
	var b bytes.Buffer
	formatAmbiguous(&b, "Stil", []gamedb.Player{
		{FLSID: "A1", CharacterName: "Stilgar", OnlineStatus: "Offline"},
		{FLSID: "A2", CharacterName: "Stilburn", OnlineStatus: "Online"},
	})
	out := b.String()
	for _, want := range []string{"Stil", "Stilgar", "Stilburn", "A1", "A2"} {
		if !strings.Contains(out, want) {
			t.Errorf("ambiguous output missing %q:\n%s", want, out)
		}
	}
}

func TestHasFlag(t *testing.T) {
	if !hasFlag([]string{"--top", "5", "--id"}, "--id") {
		t.Fatal("--id should be detected")
	}
	if hasFlag([]string{"--raw"}, "--id") {
		t.Fatal("--id absent")
	}
}
