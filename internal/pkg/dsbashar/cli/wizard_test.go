package cli

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dsbashar/config"
)

func reader(s string) *bufio.Reader { return bufio.NewReader(strings.NewReader(s)) }

func TestResolve_Case1_FlagsCompleteNoConfig(t *testing.T) {
	in := resolveInput{
		Flags:        config.Override{WorldName: "Arrakis", WorldRegion: "Europe", ServerDisplayName: "S"},
		FLSTokenFlag: "/t/fls", BGNameFlag: "Arrakis",
	}
	got, err := resolveConfig(in, reader(""), io.Discard)
	if err != nil {
		t.Fatalf("case1: %v", err)
	}
	if got.BGName != "Arrakis" || got.Cfg.WorldRegion != "Europe" || got.FLSTokenPath != "/t/fls" {
		t.Fatalf("case1 resolved = %+v", got)
	}
}

func TestResolve_Case2_NoFlagsNoConfig_DeclineWizard_Aborts(t *testing.T) {
	in := resolveInput{} // nothing
	_, err := resolveConfig(in, reader("n\n"), io.Discard)
	if !errors.Is(err, errAbort) {
		t.Fatalf("declining the wizard must abort with doctor hint, got %v", err)
	}
}

func TestResolve_Case2_NoTTY_Aborts(t *testing.T) {
	in := resolveInput{NoInput: true}
	_, err := resolveConfig(in, reader(""), io.Discard)
	if !errors.Is(err, errAbort) {
		t.Fatalf("no-input must abort, got %v", err)
	}
}

func TestResolve_Case4_ConfigNoFlags_UsesCluster(t *testing.T) {
	found := config.Config{Target: config.TargetProd, WorldName: "Arrakis", WorldRegion: "Europe"}
	in := resolveInput{Found: &found, FoundExists: true, BGNameFlag: "Arrakis", FLSTokenFlag: "/t/fls"}
	got, err := resolveConfig(in, reader(""), io.Discard)
	if err != nil {
		t.Fatalf("case4: %v", err)
	}
	if got.Cfg.WorldName != "Arrakis" {
		t.Fatalf("case4 should use cluster config, got %+v", got.Cfg)
	}
}

func TestResolve_Case3_ConflictShowsDiff_DeclineAborts(t *testing.T) {
	found := config.Config{WorldName: "Old", WorldRegion: "Europe"}
	in := resolveInput{Flags: config.Override{WorldName: "New"}, Found: &found, FoundExists: true,
		BGNameFlag: "New", FLSTokenFlag: "/t/fls"}
	var out strings.Builder
	_, err := resolveConfig(in, reader("n\n"), &out)
	if !errors.Is(err, errAbort) {
		t.Fatalf("conflict-decline must abort, got %v", err)
	}
	if !strings.Contains(out.String(), "WorldName") {
		t.Fatalf("diff should name the conflicting key, got %q", out.String())
	}
}

func TestPromptValue_EmptyKeepsDefault(t *testing.T) {
	var out strings.Builder
	got := promptValue(reader("\n"), &out, "World name", "Arrakis")
	if got != "Arrakis" {
		t.Fatalf("empty input should keep default %q, got %q", "Arrakis", got)
	}
}

func TestDiffKeys_ReportsHostDatacenterID(t *testing.T) {
	flags := config.Override{HostDatacenterID: "bg.example.test"}
	found := config.Config{HostDatacenterID: "dune-testing"}
	got := diffKeys(flags, found)
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "HostDatacenterID") {
		t.Fatalf("diffKeys did not report HostDatacenterID conflict: %v", got)
	}
	// matching value → no conflict
	if d := diffKeys(config.Override{HostDatacenterID: "dune-testing"}, found); len(d) != 0 {
		t.Fatalf("matching value reported a conflict: %v", d)
	}
}
