package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	in := &Config{Targets: []Target{{
		Name:      "vm-dune-01",
		Recipient: "age1qxyz",
		Binaries:  []string{"ds-bashar"},
		Secrets:   map[string]string{"fls-token": "secrets/vm-dune-01/fls-token.age"},
	}}}
	if err := in.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// file perms 0600, dir 0700
	fi, _ := os.Stat(filepath.Join(dir, "config.yaml"))
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("config.yaml mode = %o, want 600", fi.Mode().Perm())
	}
	out, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(out.Targets) != 1 || out.Targets[0].Name != "vm-dune-01" ||
		out.Targets[0].Secrets["fls-token"] != "secrets/vm-dune-01/fls-token.age" {
		t.Fatalf("roundtrip mismatch: %+v", out.Targets)
	}
}

func TestLoadMissingReturnsEmpty(t *testing.T) {
	out, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if len(out.Targets) != 0 {
		t.Fatalf("want empty config, got %+v", out)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		ok   bool
	}{
		{"good", Config{Targets: []Target{{Name: "a", Recipient: "age1abc", Kind: "prod", Binaries: []string{"ds-bashar"}, Secrets: map[string]string{"fls-token": "x.age"}}}}, true},
		{"empty recipient ok (pre-init)", Config{Targets: []Target{{Name: "a"}}}, true},
		{"no name", Config{Targets: []Target{{Recipient: "age1abc"}}}, false},
		{"bad recipient", Config{Targets: []Target{{Name: "a", Recipient: "nope"}}}, false},
		{"unknown binary", Config{Targets: []Target{{Name: "a", Binaries: []string{"evil"}}}}, false},
		{"unknown secret key", Config{Targets: []Target{{Name: "a", Secrets: map[string]string{"bad": "x.age"}}}}, false},
		{"fls-token without kind", Config{Targets: []Target{{Name: "a", Secrets: map[string]string{"fls-token": "x.age"}}}}, false},
		{"bad kind", Config{Targets: []Target{{Name: "a", Kind: "staging"}}}, false},
	}
	for _, tc := range cases {
		err := tc.cfg.Validate()
		if (err == nil) != tc.ok {
			t.Errorf("%s: Validate() err=%v, want ok=%v", tc.name, err, tc.ok)
		}
	}
}

func TestConfigDirHonorsXDG(t *testing.T) {
	get := func(k string) string {
		if k == "XDG_CONFIG_HOME" {
			return "/tmp/xdg"
		}
		return ""
	}
	got, err := ConfigDir(get)
	if err != nil || got != "/tmp/xdg/dapdsm" {
		t.Fatalf("ConfigDir = %q, %v; want /tmp/xdg/dapdsm", got, err)
	}
}
