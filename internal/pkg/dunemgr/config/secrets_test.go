package config

import (
	"path/filepath"
	"testing"

	"filippo.io/age"
)

func TestSecretsRoundTrip(t *testing.T) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("GenerateX25519Identity: %v", err)
	}
	dir := t.TempDir()
	store := SecretsStore{
		Path:       filepath.Join(dir, "secrets.json.age"),
		Recipients: []age.Recipient{id.Recipient()},
		Identities: []age.Identity{id},
	}

	in := Secrets{
		FLSTokens: map[string]string{"prod": "JWT-DATA"},
	}
	if err := store.Save(in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out.FLSTokens["prod"] != "JWT-DATA" {
		t.Errorf("FLSTokens[prod] = %q, want JWT-DATA", out.FLSTokens["prod"])
	}
}

func TestSecretsLoadMissingIsEmpty(t *testing.T) {
	id, _ := age.GenerateX25519Identity()
	dir := t.TempDir()
	store := SecretsStore{
		Path:       filepath.Join(dir, "secrets.json.age"),
		Recipients: []age.Recipient{id.Recipient()},
		Identities: []age.Identity{id},
	}
	out, err := store.Load()
	if err != nil {
		t.Fatalf("Load on missing file: %v", err)
	}
	if len(out.FLSTokens) != 0 {
		t.Errorf("missing file should yield empty Secrets, got %+v", out)
	}
}
