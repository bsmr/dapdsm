package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
)

// osGetenv is the single environment hook for the CLI package; tests may
// override it, though most use t.Setenv against the real environment.
var osGetenv = os.Getenv

// configDir returns the dunemgr config directory:
// $XDG_CONFIG_HOME/dunemgr, or ~/.config/dunemgr when XDG is unset.
func configDir() (string, error) {
	xdg := osGetenv("XDG_CONFIG_HOME")
	if xdg == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("home: %w", err)
		}
		xdg = filepath.Join(home, ".config")
	}
	return filepath.Join(xdg, "dunemgr"), nil
}

// dataDir returns the dunemgr data directory: $XDG_DATA_HOME/dunemgr, or
// the config directory when XDG_DATA_HOME is unset.
func dataDir() (string, error) {
	if xdg := osGetenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "dunemgr"), nil
	}
	return configDir()
}

// openStore opens the bbolt-backed state store under the data directory.
// The caller owns Close.
func openStore() (*store.Store, error) {
	dir, err := dataDir()
	if err != nil {
		return nil, err
	}
	return store.Open(filepath.Join(dir, "state.bolt"))
}
