package cli

import (
	"bytes"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/config"
)

func TestWizardGathersTargetFromPrompts(t *testing.T) {
	// Kind: "prod"; Binaries: "ds-bashar".
	in := strings.NewReader("prod\nds-bashar\n")
	var out bytes.Buffer
	tgt, err := wizardTarget(in, &out, "vm-a", "age1recip")
	if err != nil {
		t.Fatalf("wizardTarget: %v", err)
	}
	if tgt.Name != "vm-a" || tgt.Recipient != "age1recip" {
		t.Fatalf("target identity wrong: %+v", tgt)
	}
	if tgt.Kind != "prod" {
		t.Fatalf("kind = %q, want prod", tgt.Kind)
	}
	if len(tgt.Binaries) != 1 || tgt.Binaries[0] != "ds-bashar" {
		t.Fatalf("binaries = %v", tgt.Binaries)
	}
	if err := (&config.Config{Targets: []config.Target{tgt}}).Validate(); err != nil {
		t.Fatalf("wizard produced invalid target: %v", err)
	}
}
