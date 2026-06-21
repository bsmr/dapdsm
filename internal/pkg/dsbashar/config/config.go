// Package config loads ds-bashar operator configuration from
// /etc/dune/dunectl.env, a sourceable POSIX shell KEY=VALUE file.
//
// The same file is consumed by Bash helpers (via `. /etc/dune/dunectl.env`)
// and by ds-bashar (via Parse). Both sides must read identical semantics, so
// the parser implements only what POSIX shell sourcing would also accept:
// blank lines, # comments, KEY=VALUE with optional matched outer quotes,
// no escape sequences.
package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// DefaultPath is the on-disk location of the ds-bashar operator config.
const DefaultPath = "/etc/dune/dunectl.env"

// Steam app IDs for the two supported targets.
const (
	AppIDProd uint32 = 4754530
	AppIDTest uint32 = 3104830
)

// Target identifies which Funcom server pool a host serves.
type Target string

const (
	TargetProd Target = "prod"
	TargetTest Target = "test"
)

// Config is the parsed contents of /etc/dune/dunectl.env.
type Config struct {
	Target          Target
	FLSTokenFile    string
	K3SExtraTLSSANs []string
	// WorldName is the BattleGroup identifier passed to Funcom's
	// setup.sh as `world_name`. Must be YAML-safe (alphanumeric plus
	// underscore/hyphen) — the generated CR sets it as a bare scalar.
	WorldName string
	// WorldRegion is the human region name ("Europe", "North America", …).
	// ds-bashar maps it to the 1-based index Funcom's setup.sh expects.
	WorldRegion string
	// HostDatacenterID is the short identifier patched into the
	// HOST_DATACENTER_ID env-var on the Director / Server-Gateway /
	// Text-Router pods. Empty (the default) leaves the Funcom vendor
	// default "dune-testing" in place.
	HostDatacenterID string
	// Reconcile knobs: read by `ds-bashar reconcile` to drive the
	// post-bootstrap pipeline declaratively without a shell script.
	AlwaysOnSets       []string // ALWAYS_ON_SETS — space-separated map names
	ServerDisplayName  string   // SERVER_DISPLAY_NAME
	ServerPasswordFile string   // SERVER_PASSWORD_FILE
	GamePortBase       int      // GAME_PORT_BASE (0 = leave Funcom default)
	IGWPortBase        int      // IGW_PORT_BASE  (0 = leave Funcom default)
	SkipInitDB         bool     // SKIP_INIT_DB (default false → run init-db)
}

// Funcom's setup/world.sh enumerates these regions in this order; the
// number passed to setup.sh is the 1-based index into the slice.
var worldRegions = []string{"Asia", "Europe", "North America", "Oceania", "South America"}

// RegionNumber returns the 1-based index for a human region name.
// Match is case-insensitive but otherwise exact. Returns 0 + error for
// any name not in the Funcom-supplied list.
func RegionNumber(name string) (int, error) {
	for i, r := range worldRegions {
		if strings.EqualFold(name, r) {
			return i + 1, nil
		}
	}
	return 0, fmt.Errorf("unknown region %q: pick one of %s", name, strings.Join(worldRegions, ", "))
}

// maxWorldNameLen is Funcom's BattleGroup-title limit (world.sh caps it ~50).
const maxWorldNameLen = 50

// ValidWorldName reports whether name is acceptable as the BattleGroup title.
// The title is rendered as a YAML double-quoted, escaped scalar (see
// worldsetup.yamlQuote) and piped to Funcom's setup prompt, so spaces, colons,
// dots and slashes are all safe — Funcom prompts for it interactively and the
// server browser shows such names ("Test Server", "Dune Awakening powered by
// no-ruto.net"). The only constraints are Funcom's ~50-rune limit and no
// control characters (a newline would break the YAML line / the stdin prompt).
//
// Quoting the rendered value is what makes this safe: an earlier bare-scalar
// render broke on titles containing a colon-space (`: `), which the regexp
// whitelist over-corrected by also banning spaces.
func ValidWorldName(name string) bool {
	if name == "" || utf8.RuneCountInString(name) > maxWorldNameLen {
		return false
	}
	for _, r := range name {
		// unicode.IsControl covers C0/C1/DEL; U+2028/U+2029 are YAML 1.2 line
		// breaks that IsControl misses and yamlQuote does not escape — reject
		// them too so the rendered CR always applies.
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return false
		}
	}
	return true
}

// Parse reads a dunectl.env stream and returns the resulting Config.
//
// Unknown keys are ignored so newer shell-side settings do not break older
// ds-bashar binaries.
func Parse(r io.Reader) (Config, error) {
	var c Config
	scanner := bufio.NewScanner(r)
	for line := 1; scanner.Scan(); line++ {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		key, value, ok := strings.Cut(raw, "=")
		if !ok {
			return Config{}, fmt.Errorf("line %d: missing '='", line)
		}
		value = unquote(strings.TrimSpace(value))
		switch strings.TrimSpace(key) {
		case "TARGET":
			c.Target = Target(value)
		case "FLS_TOKEN_FILE":
			c.FLSTokenFile = value
		case "K3S_EXTRA_TLS_SAN":
			c.K3SExtraTLSSANs = strings.Fields(value)
		case "WORLD_NAME":
			c.WorldName = value
		case "WORLD_REGION":
			c.WorldRegion = value
		case "HOST_DATACENTER_ID":
			c.HostDatacenterID = value
		case "ALWAYS_ON_SETS":
			c.AlwaysOnSets = strings.Fields(value)
		case "SERVER_DISPLAY_NAME":
			c.ServerDisplayName = value
		case "SERVER_PASSWORD_FILE":
			c.ServerPasswordFile = value
		case "GAME_PORT_BASE":
			if n, err := parsePositiveInt(value); err == nil {
				c.GamePortBase = n
			} else if value != "" {
				return Config{}, fmt.Errorf("line %d: GAME_PORT_BASE %q: %w", line, value, err)
			}
		case "IGW_PORT_BASE":
			if n, err := parsePositiveInt(value); err == nil {
				c.IGWPortBase = n
			} else if value != "" {
				return Config{}, fmt.Errorf("line %d: IGW_PORT_BASE %q: %w", line, value, err)
			}
		case "SKIP_INIT_DB":
			c.SkipInitDB = parseBool(value)
		}
	}
	if err := scanner.Err(); err != nil {
		return Config{}, err
	}
	if err := c.validate(); err != nil {
		return Config{}, err
	}
	c.applyDefaults()
	return c, nil
}

func (c *Config) validate() error {
	switch c.Target {
	case TargetProd, TargetTest:
		return nil
	case "":
		return fmt.Errorf("TARGET is required")
	default:
		return fmt.Errorf("TARGET %q: want %q or %q", c.Target, TargetProd, TargetTest)
	}
}

func (c *Config) applyDefaults() {
	if c.FLSTokenFile == "" {
		c.FLSTokenFile = "/etc/dune/fls-token-" + string(c.Target)
	}
}

// AppID returns the Steam app id for the configured target.
// Returns 0 for an unset/unknown Target — validate() prevents that case
// for any Config returned from Parse.
func (c Config) AppID() uint32 {
	switch c.Target {
	case TargetProd:
		return AppIDProd
	case TargetTest:
		return AppIDTest
	}
	return 0
}

// LoadFromFile reads and parses the ds-bashar env-file at path.
func LoadFromFile(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	return Parse(f)
}

func parsePositiveInt(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("not an integer")
	}
	if n < 0 {
		return 0, fmt.Errorf("must be >= 0")
	}
	return n, nil
}

// parseBool accepts "1"/"true"/"yes"/"on" (case-insensitive). Anything
// else — including the empty string — is false.
func parseBool(s string) bool {
	switch strings.ToLower(s) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func unquote(s string) string {
	if len(s) >= 2 {
		first, last := s[0], s[len(s)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
