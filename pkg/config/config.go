// Package config is the typed workstation source-of-truth for the dapdsm tool
// suite: rollout targets, their age recipient, the binaries to ship, and
// references to sealed secret files. It is additive — it never reads, writes,
// or migrates the on-VM dunectl.env, the K3s/RKE2 config, or the dunemgr YAML.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

// KnownBinaries are the suite binaries a target may roll out.
var KnownBinaries = map[string]bool{"ds-bashar": true, "ds-arrakis": true}

// KnownSecretKeys are the secret refs a target may carry. Each maps to an
// on-VM destination in pkg/domain/rollout.
var KnownSecretKeys = map[string]bool{"fls-token": true, "server-password": true}

var recipientRE = regexp.MustCompile(`^age1[0-9a-z]+$`)

// Config is the YAML-backed workstation config at <ConfigDir>/config.yaml.
type Config struct {
	Targets []Target `yaml:"targets"`
}

// Target is one rollout destination.
type Target struct {
	Name      string            `yaml:"name"`               // SSH host/alias (resolved via ~/.ssh/config)
	Recipient string            `yaml:"recipient"`          // VM age public recipient (cached by `init`)
	Kind      string            `yaml:"kind,omitempty"`     // prod|test — required when fls-token secret is present
	Binaries  []string          `yaml:"binaries"`           // suite binaries to roll out
	Secrets   map[string]string `yaml:"secrets,omitempty"`  // secret key -> sealed .age path (relative to ConfigDir)
}

// ConfigDir returns $XDG_CONFIG_HOME/dapdsm, or ~/.config/dapdsm.
func ConfigDir(getenv func(string) string) (string, error) {
	if xdg := getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "dapdsm"), nil
	}
	home := getenv("HOME")
	if home == "" {
		return "", fmt.Errorf("config dir: neither XDG_CONFIG_HOME nor HOME set")
	}
	return filepath.Join(home, ".config", "dapdsm"), nil
}

// Load reads <dir>/config.yaml; a missing file yields an empty Config.
func Load(dir string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &c, nil
}

// Save writes <dir>/config.yaml (dir 0700, file 0600).
func (c *Config) Save(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir config: %w", err)
	}
	out, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), out, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// Target returns the named target.
func (c *Config) Target(name string) (*Target, bool) {
	for i := range c.Targets {
		if c.Targets[i].Name == name {
			return &c.Targets[i], true
		}
	}
	return nil, false
}

// Validate checks each target: non-empty name, age1-format recipient (if set),
// known binaries, known secret keys, and that Kind is valid when set. Targets
// carrying the fls-token secret must have Kind set so the on-VM path is
// unambiguous (/etc/dune/fls-token-prod vs /etc/dune/fls-token-test).
func (c *Config) Validate() error {
	for _, t := range c.Targets {
		if t.Name == "" {
			return fmt.Errorf("target with empty name")
		}
		if t.Recipient != "" && !recipientRE.MatchString(t.Recipient) {
			return fmt.Errorf("target %s: invalid age recipient %q", t.Name, t.Recipient)
		}
		if t.Kind != "" && t.Kind != "prod" && t.Kind != "test" {
			return fmt.Errorf("target %s: invalid kind %q (must be prod or test)", t.Name, t.Kind)
		}
		for _, b := range t.Binaries {
			if !KnownBinaries[b] {
				return fmt.Errorf("target %s: unknown binary %q", t.Name, b)
			}
		}
		for k := range t.Secrets {
			if !KnownSecretKeys[k] {
				return fmt.Errorf("target %s: unknown secret key %q", t.Name, k)
			}
			if k == "fls-token" && t.Kind == "" {
				return fmt.Errorf("target %s: secret fls-token requires kind (prod|test) to be set", t.Name)
			}
		}
	}
	return nil
}
