package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
)

const tokenBytes = 32

// EnsureToken returns the contents of <configDir>/token. If the
// file is missing, a fresh base64url-encoded random token is
// generated and written with mode 0600.
func EnsureToken(configDir string) (string, error) {
	path := filepath.Join(configDir, "token")
	data, err := os.ReadFile(path)
	if err == nil {
		return string(data), nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("read token: %w", err)
	}
	return RegenToken(configDir)
}

// RegenToken writes a fresh random token to <configDir>/token,
// overwriting any existing file. Returns the new token.
func RegenToken(configDir string) (string, error) {
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	tok := base64.RawURLEncoding.EncodeToString(buf)
	path := filepath.Join(configDir, "token")
	if err := os.WriteFile(path, []byte(tok), 0o600); err != nil {
		return "", fmt.Errorf("write: %w", err)
	}
	return tok, nil
}
