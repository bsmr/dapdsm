// Package gameini exposes a curated allowlist of known Funcom gameplay settings
// in the vendor UserEngine.ini, read/set over SSH. It reuses internal/pkg/iniconf
// for comment-preserving edits. Out of scope: per-partition UserGame.ini.
package gameini

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go.muehmer.eu/dapdsm/internal/pkg/iniconf"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// VendorEngineINI is the canonical vendor UserEngine.ini path on the host.
const VendorEngineINI = "/home/dune/.dune/download/scripts/setup/config/UserEngine.ini"

// Kind classifies an allowlisted setting's value type for validation.
type Kind int

const (
	KindBool Kind = iota
	KindInt
	KindFloat
	KindString
)

// Setting describes one allowlisted key.
type Setting struct {
	Key     string
	Section string
	File    string
	Kind    Kind
}

var allowlist = []Setting{
	{"Dune.GlobalMiningOutputMultiplier", "ConsoleVariables", VendorEngineINI, KindFloat},
	{"Dune.GlobalVehicleMiningOutputMultiplier", "ConsoleVariables", VendorEngineINI, KindFloat},
	{"SecurityZones.PvpResourceMultiplier", "ConsoleVariables", VendorEngineINI, KindFloat},
	{"Sandstorm.Enabled", "ConsoleVariables", VendorEngineINI, KindInt},
	{"Sandstorm.AutoSpawn", "ConsoleVariables", VendorEngineINI, KindBool},
	{"Sandstorm.CoriolisAutoSpawnEnabled", "ConsoleVariables", VendorEngineINI, KindBool},
	{"sandworm.dune.Enabled", "ConsoleVariables", VendorEngineINI, KindInt},
	{"Sandworm.SandwormDangerZonesEnabled", "ConsoleVariables", VendorEngineINI, KindBool},
	{"Vehicle.SandwormInvulnerabilitySecondsOnExit", "ConsoleVariables", VendorEngineINI, KindFloat},
	{"Vehicle.SandwormInvulnerabilitySecondsOnServerRestart", "ConsoleVariables", VendorEngineINI, KindFloat},
	{"dw.VehicleDurabilityDamageMultiplier", "ConsoleVariables", VendorEngineINI, KindFloat},
	{"Bgd.ServerDisplayName", "ConsoleVariables", VendorEngineINI, KindString},
}

var byKey = func() map[string]Setting {
	m := make(map[string]Setting, len(allowlist))
	for _, s := range allowlist {
		m[s.Key] = s
	}
	return m
}()

// Keys returns the allowlisted keys, sorted (for TUI completion + help).
func Keys() []string {
	out := make([]string, 0, len(allowlist))
	for _, s := range allowlist {
		out = append(out, s.Key)
	}
	sort.Strings(out)
	return out
}

// Lookup returns the Setting for key.
func Lookup(key string) (Setting, bool) { s, ok := byKey[key]; return s, ok }

// ValidateValue checks value against kind.
func ValidateValue(kind Kind, value string) error {
	switch kind {
	case KindBool:
		switch value {
		case "true", "false", "True", "False":
			return nil
		}
		return fmt.Errorf("expected bool (true/false), got %q", value)
	case KindInt:
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			return fmt.Errorf("expected integer, got %q", value)
		}
	case KindFloat:
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("expected number, got %q", value)
		}
	case KindString:
		if value == "" {
			return fmt.Errorf("string value must not be empty")
		}
	}
	return nil
}

// Runner reads/sets allowlisted settings over SSH.
type Runner struct {
	SSH      *ssh.Client
	BGBinary string
}

func (r *Runner) bg() string {
	if r.BGBinary != "" {
		return r.BGBinary
	}
	return "/home/dune/.dune/bin/battlegroup"
}

func (r *Runner) readFile(ctx context.Context, host, path string) ([]byte, error) {
	res, err := r.SSH.Run(ctx, host, "cat", path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("read %s: exit %d: %s", path, res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	return []byte(res.Stdout), nil
}

// Get returns the current value of an allowlisted key ("" if unset).
func (r *Runner) Get(ctx context.Context, host, key string) (string, error) {
	s, ok := Lookup(key)
	if !ok {
		return "", fmt.Errorf("unknown setting %q (not in the allowlist)", key)
	}
	content, err := r.readFile(ctx, host, s.File)
	if err != nil {
		return "", err
	}
	return currentValue(content, s.Section, s.Key), nil
}

// KV is a key and its current value.
type KV struct {
	Key   string
	Value string
}

// List returns the allowlist with each key's current value.
func (r *Runner) List(ctx context.Context, host string) ([]KV, error) {
	content, err := r.readFile(ctx, host, VendorEngineINI)
	if err != nil {
		return nil, err
	}
	var out []KV
	for _, key := range Keys() {
		s, _ := Lookup(key)
		out = append(out, KV{Key: key, Value: currentValue(content, s.Section, key)})
	}
	return out, nil
}

// Set validates and writes key=value, optionally applying + restarting.
func (r *Runner) Set(ctx context.Context, host, key, value string, apply, restart bool) error {
	s, ok := Lookup(key)
	if !ok {
		return fmt.Errorf("unknown setting %q (not in the allowlist)", key)
	}
	if err := ValidateValue(s.Kind, value); err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	content, err := r.readFile(ctx, host, s.File)
	if err != nil {
		return err
	}
	written := value
	if iniconf.NeedsQuoting(written) {
		written = iniconf.Quote(written)
	}
	updated, err := iniconf.SetKey(content, s.Section, s.Key, written)
	if err != nil {
		return err
	}
	wres, err := r.SSH.RunWithStdin(ctx, host, updated, "tee", s.File)
	if err != nil {
		return fmt.Errorf("write %s: %w", s.File, err)
	}
	if wres.ExitCode != 0 {
		return fmt.Errorf("write %s: exit %d: %s", s.File, wres.ExitCode, strings.TrimSpace(wres.Stderr))
	}
	if apply {
		ares, aerr := r.SSH.Run(ctx, host, r.bg(), "apply-default-usersettings")
		if aerr != nil {
			return fmt.Errorf("apply-default-usersettings failed: %w", aerr)
		}
		if ares.ExitCode != 0 {
			return fmt.Errorf("apply-default-usersettings failed: exit %d: %s", ares.ExitCode, strings.TrimSpace(ares.Stderr))
		}
	}
	if restart {
		rres, rerr := r.SSH.Run(ctx, host, r.bg(), "restart")
		if rerr != nil {
			return fmt.Errorf("battlegroup restart failed: %w", rerr)
		}
		if rres.ExitCode != 0 {
			return fmt.Errorf("battlegroup restart failed: exit %d: %s", rres.ExitCode, strings.TrimSpace(rres.Stderr))
		}
	}
	return nil
}

// currentValue extracts a live key's value from INI content ("" if absent).
func currentValue(content []byte, section, key string) string {
	inSection := false
	want := "[" + section + "]"
	for _, line := range strings.Split(string(content), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			inSection = t == want
			continue
		}
		if inSection && strings.HasPrefix(t, key+"=") {
			return strings.TrimSpace(strings.TrimPrefix(t, key+"="))
		}
	}
	return ""
}
