// Package config owns paths and file I/O for ~/.config/dunemgr.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the YAML-backed operator preferences file. New fields
// default to zero-value; missing fields in existing files use
// these defaults.
type Config struct {
	Bind        string `yaml:"bind"`         // default "127.0.0.1:8765"
	DefaultHost string `yaml:"default_host"` // default ""
}

func defaults() Config {
	return Config{
		Bind: "127.0.0.1:8765",
	}
}

// Load reads (or creates) config.yaml in configDir.
func Load(configDir string) (Config, error) {
	path := filepath.Join(configDir, "config.yaml")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return Config{}, fmt.Errorf("mkdir: %w", err)
	}
	cfg := defaults()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// First run: write defaults.
		out, mErr := yaml.Marshal(cfg)
		if mErr != nil {
			return Config{}, fmt.Errorf("marshal default: %w", mErr)
		}
		if wErr := os.WriteFile(path, out, 0o600); wErr != nil {
			return Config{}, fmt.Errorf("write default: %w", wErr)
		}
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("yaml: %w", err)
	}
	return cfg, nil
}
