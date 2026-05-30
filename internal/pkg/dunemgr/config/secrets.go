package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"filippo.io/age"
)

// Secrets is the plaintext shape of secrets.json.age.
type Secrets struct {
	FLSTokens     map[string]string `json:"fls_tokens,omitempty"`     // per TARGET
	PGCredentials map[string]string `json:"pg_credentials,omitempty"` // per host — reserved for v1.x
}

// SecretsStore wraps the encrypted file on disk.
type SecretsStore struct {
	Path       string
	Recipients []age.Recipient // for encrypt
	Identities []age.Identity  // for decrypt
}

// Save encrypts and writes the secrets to Path with mode 0600.
func (s SecretsStore) Save(sec Secrets) error {
	plain, err := json.Marshal(sec)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	var ciphertext bytes.Buffer
	w, err := age.Encrypt(&ciphertext, s.Recipients...)
	if err != nil {
		return fmt.Errorf("age encrypt init: %w", err)
	}
	if _, err := w.Write(plain); err != nil {
		return fmt.Errorf("age write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("age close: %w", err)
	}
	return os.WriteFile(s.Path, ciphertext.Bytes(), 0o600)
}

// Load decrypts and parses the file. Missing file yields a
// zero-value Secrets with no error.
func (s SecretsStore) Load() (Secrets, error) {
	data, err := os.ReadFile(s.Path)
	if os.IsNotExist(err) {
		return Secrets{}, nil
	}
	if err != nil {
		return Secrets{}, fmt.Errorf("read: %w", err)
	}
	r, err := age.Decrypt(bytes.NewReader(data), s.Identities...)
	if err != nil {
		return Secrets{}, fmt.Errorf("age decrypt: %w", err)
	}
	plain, err := io.ReadAll(r)
	if err != nil {
		return Secrets{}, fmt.Errorf("read decrypted: %w", err)
	}
	var sec Secrets
	if err := json.Unmarshal(plain, &sec); err != nil {
		return Secrets{}, fmt.Errorf("unmarshal: %w", err)
	}
	return sec, nil
}
