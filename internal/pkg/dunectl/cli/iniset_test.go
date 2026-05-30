package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

const iniBefore = `[ConsoleVariables]
;Bgd.ServerDisplayName="My Arrakis, My Dune"
;Bgd.ServerLoginPassword="Sandworm"
Dune.GlobalMiningOutputMultiplier=1.0
`

func newIniDeps(initial string) (*string, *[]string, iniSetDeps) {
	current := initial
	var vendorCalls []string
	deps := iniSetDeps{
		readFile: func(_ string) ([]byte, error) {
			return []byte(current), nil
		},
		writeFile: func(_ string, content []byte, _ os.FileMode) error {
			current = string(content)
			return nil
		},
		runVendor: func(_ context.Context, _ string, action string, _, _ io.Writer) error {
			vendorCalls = append(vendorCalls, action)
			return nil
		},
	}
	return &current, &vendorCalls, deps
}

func TestIniSet_AutoQuotesStringValue(t *testing.T) {
	t.Parallel()
	current, _, deps := newIniDeps(iniBefore)
	var stdout, stderr bytes.Buffer
	err := iniSet(context.Background(), []string{"Bgd.ServerDisplayName", "Schleifweg"}, &stdout, &stderr, deps)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(*current, `Bgd.ServerDisplayName="Schleifweg"`) {
		t.Errorf("missing quoted value in:\n%s", *current)
	}
	if !strings.Contains(stdout.String(), `=`+`"Schleifweg"`) {
		t.Errorf("stdout summary did not echo quoted value: %q", stdout.String())
	}
}

func TestIniSet_RawSkipsAutoQuote(t *testing.T) {
	t.Parallel()
	current, _, deps := newIniDeps(iniBefore)
	var stdout, stderr bytes.Buffer
	err := iniSet(context.Background(),
		[]string{"--raw", "Dune.GlobalMiningOutputMultiplier", "2"},
		&stdout, &stderr, deps)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(*current, `Dune.GlobalMiningOutputMultiplier=2`) {
		t.Errorf("expected raw value:\n%s", *current)
	}
}

func TestIniSet_ApplyRunsVendor(t *testing.T) {
	t.Parallel()
	_, vendorCalls, deps := newIniDeps(iniBefore)
	var stdout, stderr bytes.Buffer
	err := iniSet(context.Background(),
		[]string{"--apply", "Bgd.ServerLoginPassword", "A123456!"},
		&stdout, &stderr, deps)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(*vendorCalls) != 1 || (*vendorCalls)[0] != "apply-default-usersettings" {
		t.Errorf("vendorCalls = %v, want [apply-default-usersettings]", *vendorCalls)
	}
}

func TestIniSet_RestartImpliesApply(t *testing.T) {
	t.Parallel()
	_, vendorCalls, deps := newIniDeps(iniBefore)
	var stdout, stderr bytes.Buffer
	err := iniSet(context.Background(),
		[]string{"--restart", "Bgd.ServerLoginPassword", "secret"},
		&stdout, &stderr, deps)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := []string{"apply-default-usersettings", "restart"}
	if len(*vendorCalls) != 2 || (*vendorCalls)[0] != want[0] || (*vendorCalls)[1] != want[1] {
		t.Errorf("vendorCalls = %v, want %v", *vendorCalls, want)
	}
}

func TestIniSet_CustomFileAndSection(t *testing.T) {
	t.Parallel()
	_, _, deps := newIniDeps(iniBefore)
	var stdout, stderr bytes.Buffer
	err := iniSet(context.Background(),
		[]string{
			"--file", "/tmp/whatever.ini",
			"--section", "/Script/DuneSandbox.PvpPveSettings",
			"m_bShouldForceEnablePvpOnAllPartitions", "True",
		},
		&stdout, &stderr, deps)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stdout.String(), "[/Script/DuneSandbox.PvpPveSettings]") {
		t.Errorf("stdout summary did not include section: %q", stdout.String())
	}
}

func TestIniSet_AcceptsFlagsAfterPositionals(t *testing.T) {
	t.Parallel()
	// User-pattern from 2026-05-25: `dunectl ini-set KEY VALUE --apply --restart`.
	// Go's flag package stops at the first non-flag, so without the
	// pre-reorder this used to fail with "need exactly two positional args".
	_, _, deps := newIniDeps(iniBefore)
	var stdout, stderr bytes.Buffer
	err := iniSet(context.Background(),
		[]string{"Bgd.ServerLoginPassword", "geheim", "--apply", "--restart"},
		&stdout, &stderr, deps)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestIniSet_NotEnoughArgsReturnsErrUsage(t *testing.T) {
	t.Parallel()
	_, _, deps := newIniDeps(iniBefore)
	var stdout, stderr bytes.Buffer
	err := iniSet(context.Background(), []string{"only-one-arg"}, &stdout, &stderr, deps)
	if !errors.Is(err, ErrUsage) {
		t.Errorf("err = %v, want errors.Is(err, ErrUsage)", err)
	}
}
