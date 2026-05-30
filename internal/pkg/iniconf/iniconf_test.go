package iniconf

import (
	"strings"
	"testing"
)

const engineINI = `; Header comment
[ConsoleVariables]
; Set the name
;Bgd.ServerDisplayName="My Arrakis, My Dune"

; Set a password
;Bgd.ServerLoginPassword="Sandworm"

; Mining multipliers
Dune.GlobalMiningOutputMultiplier=1.0
SecurityZones.PvpResourceMultiplier=2.5

[Other]
ExistingKey=already-set
`

func TestSetKey_UncommentsAndReplaces(t *testing.T) {
	t.Parallel()
	out, err := SetKey([]byte(engineINI), "ConsoleVariables", "Bgd.ServerDisplayName", `"Schleifweg"`)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	got := string(out)
	if !strings.Contains(got, `Bgd.ServerDisplayName="Schleifweg"`) {
		t.Errorf("missing target line in:\n%s", got)
	}
	if strings.Contains(got, `;Bgd.ServerDisplayName=`) {
		t.Errorf("commented entry still present in:\n%s", got)
	}
}

func TestSetKey_OverwritesLiveValue(t *testing.T) {
	t.Parallel()
	out, err := SetKey([]byte(engineINI), "ConsoleVariables", "Dune.GlobalMiningOutputMultiplier", "2.5")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(string(out), "Dune.GlobalMiningOutputMultiplier=2.5") {
		t.Errorf("expected new value:\n%s", out)
	}
	if strings.Contains(string(out), "Dune.GlobalMiningOutputMultiplier=1.0") {
		t.Errorf("old value still present:\n%s", out)
	}
}

func TestSetKey_AppendsMissingKeyWithinExistingSection(t *testing.T) {
	t.Parallel()
	out, err := SetKey([]byte(engineINI), "ConsoleVariables", "Dune.NewToggle", "1")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	// New line lands inside [ConsoleVariables], before the next [section].
	lines := strings.Split(string(out), "\n")
	inSection := false
	added := false
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[Other") {
			break
		}
		if t == "[ConsoleVariables]" {
			inSection = true
			continue
		}
		if inSection && t == "Dune.NewToggle=1" {
			added = true
		}
	}
	if !added {
		t.Errorf("Dune.NewToggle=1 not in [ConsoleVariables]:\n%s", out)
	}
}

func TestSetKey_AppendsMissingSection(t *testing.T) {
	t.Parallel()
	out, err := SetKey([]byte(engineINI), "/Script/DuneSandbox.PvpPveSettings", "m_bShouldForceEnablePvpOnAllPartitions", "True")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "[/Script/DuneSandbox.PvpPveSettings]") {
		t.Errorf("expected new section header:\n%s", got)
	}
	if !strings.Contains(got, "m_bShouldForceEnablePvpOnAllPartitions=True") {
		t.Errorf("expected new key=value line:\n%s", got)
	}
	// Existing content must be preserved verbatim.
	if !strings.HasPrefix(got, "; Header comment\n[ConsoleVariables]") {
		t.Errorf("existing content modified:\n%s", got)
	}
}

func TestSetKey_PreservesUnrelatedCommentLines(t *testing.T) {
	t.Parallel()
	out, err := SetKey([]byte(engineINI), "ConsoleVariables", "Bgd.ServerLoginPassword", `"A123456!"`)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	for _, mustKeep := range []string{
		"; Header comment",
		"; Set the name",
		"; Set a password",
		"; Mining multipliers",
	} {
		if !strings.Contains(string(out), mustKeep) {
			t.Errorf("operator comment dropped: %q", mustKeep)
		}
	}
}

func TestSetKey_EmptySectionRejected(t *testing.T) {
	t.Parallel()
	if _, err := SetKey([]byte(""), "", "K", "V"); err == nil {
		t.Errorf("err = nil, want error for empty section")
	}
}

func TestNeedsQuoting(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		`Schleifweg`:       true,  // plain string
		`"already quoted"`: false, // wrapped already
		`true`:             false, // bool
		`False`:            false, // bool (UE-style cap)
		`1`:                false, // int
		`2.5`:              false, // float
		`-3.0`:             false, // negative float
		`A123456!`:         true,  // string with special chars
	}
	for v, want := range cases {
		if got := NeedsQuoting(v); got != want {
			t.Errorf("NeedsQuoting(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestQuote(t *testing.T) {
	t.Parallel()
	if Quote("x") != `"x"` {
		t.Errorf(`Quote("x") = %q, want "x"`, Quote("x"))
	}
}
