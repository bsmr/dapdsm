package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		input   string
		want    Config
		wantErr string
	}{
		{
			name:  "minimal-test",
			input: "TARGET=test\n",
			want: Config{
				Target:       TargetTest,
				FLSTokenFile: "/etc/dune/fls-token-test",
			},
		},
		{
			name:  "minimal-prod",
			input: "TARGET=prod\n",
			want: Config{
				Target:       TargetProd,
				FLSTokenFile: "/etc/dune/fls-token-prod",
			},
		},
		{
			name: "override-token-file",
			input: "TARGET=test\n" +
				"FLS_TOKEN_FILE=/srv/secrets/test.jwt\n",
			want: Config{
				Target:       TargetTest,
				FLSTokenFile: "/srv/secrets/test.jwt",
			},
		},
		{
			name: "extra-sans-double-quoted",
			input: `TARGET=prod` + "\n" +
				`K3S_EXTRA_TLS_SAN="vm-a.example.com vm-b.example.com"` + "\n",
			want: Config{
				Target:          TargetProd,
				FLSTokenFile:    "/etc/dune/fls-token-prod",
				K3SExtraTLSSANs: []string{"vm-a.example.com", "vm-b.example.com"},
			},
		},
		{
			name: "extra-sans-single-quoted",
			input: "TARGET=prod\n" +
				"K3S_EXTRA_TLS_SAN='vm-a.example.com vm-b.example.com'\n",
			want: Config{
				Target:          TargetProd,
				FLSTokenFile:    "/etc/dune/fls-token-prod",
				K3SExtraTLSSANs: []string{"vm-a.example.com", "vm-b.example.com"},
			},
		},
		{
			name: "extra-sans-unquoted-single-value",
			input: "TARGET=prod\n" +
				"K3S_EXTRA_TLS_SAN=vm-a.example.com\n",
			want: Config{
				Target:          TargetProd,
				FLSTokenFile:    "/etc/dune/fls-token-prod",
				K3SExtraTLSSANs: []string{"vm-a.example.com"},
			},
		},
		{
			name: "unknown-keys-ignored",
			input: "TARGET=test\n" +
				"FUTURE_KEY=value-that-old-binaries-do-not-understand\n",
			want: Config{
				Target:       TargetTest,
				FLSTokenFile: "/etc/dune/fls-token-test",
			},
		},
		{
			name: "comments-and-blanks-ignored",
			input: "# top comment\n" +
				"\n" +
				"   # indented comment\n" +
				"TARGET=test  \n",
			want: Config{
				Target:       TargetTest,
				FLSTokenFile: "/etc/dune/fls-token-test",
			},
		},
		{
			name:    "missing-target",
			input:   "FLS_TOKEN_FILE=/srv/t.jwt\n",
			wantErr: "TARGET is required",
		},
		{
			name:    "invalid-target",
			input:   "TARGET=staging\n",
			wantErr: `TARGET "staging"`,
		},
		{
			name:    "malformed-line-no-equals",
			input:   "TARGET test\n",
			wantErr: "missing '='",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(strings.NewReader(tc.input))
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Parse() err = %v, want substring %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() err = %v, want nil", err)
			}
			if got.Target != tc.want.Target {
				t.Errorf("Target = %q, want %q", got.Target, tc.want.Target)
			}
			if got.FLSTokenFile != tc.want.FLSTokenFile {
				t.Errorf("FLSTokenFile = %q, want %q", got.FLSTokenFile, tc.want.FLSTokenFile)
			}
			if !slices.Equal(got.K3SExtraTLSSANs, tc.want.K3SExtraTLSSANs) {
				t.Errorf("K3SExtraTLSSANs = %v, want %v", got.K3SExtraTLSSANs, tc.want.K3SExtraTLSSANs)
			}
		})
	}
}

func TestAppID(t *testing.T) {
	t.Parallel()
	cases := map[Target]uint32{
		TargetProd: AppIDProd,
		TargetTest: AppIDTest,
		Target(""): 0,
	}
	for in, want := range cases {
		if got := (Config{Target: in}).AppID(); got != want {
			t.Errorf("Config{Target:%q}.AppID() = %d, want %d", in, got, want)
		}
	}
}

func TestLoadFromFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "dunectl.env")
	if err := os.WriteFile(path, []byte("TARGET=prod\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile err = %v", err)
	}
	if cfg.Target != TargetProd {
		t.Errorf("Target = %q, want %q", cfg.Target, TargetProd)
	}
	if cfg.AppID() != AppIDProd {
		t.Errorf("AppID = %d, want %d", cfg.AppID(), AppIDProd)
	}
}

func TestDefaultPathIsDunectlEnv(t *testing.T) {
	// Spec invariant: the config file name stays /etc/dune/dunectl.env
	// (zero on-VM migration); only the binary was renamed to ds-bashar.
	if DefaultPath != "/etc/dune/dunectl.env" {
		t.Fatalf("DefaultPath = %q, want /etc/dune/dunectl.env", DefaultPath)
	}
}

func TestLoadFromFile_NotFound(t *testing.T) {
	t.Parallel()
	_, err := LoadFromFile(filepath.Join(t.TempDir(), "missing.env"))
	if err == nil || !strings.Contains(err.Error(), "open ") {
		t.Errorf("err = %v, want substring 'open '", err)
	}
}

func TestParse_WorldFields(t *testing.T) {
	t.Parallel()
	cfg, err := Parse(strings.NewReader("TARGET=prod\nWORLD_NAME=HADESNET\nWORLD_REGION=Europe\n"))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if cfg.WorldName != "HADESNET" {
		t.Errorf("WorldName = %q, want HADESNET", cfg.WorldName)
	}
	if cfg.WorldRegion != "Europe" {
		t.Errorf("WorldRegion = %q, want Europe", cfg.WorldRegion)
	}
}

func TestRegionNumber(t *testing.T) {
	t.Parallel()
	cases := map[string]int{
		"Asia":          1,
		"Europe":        2,
		"europe":        2, // case-insensitive
		"North America": 3,
		"Oceania":       4,
		"South America": 5,
	}
	for name, want := range cases {
		got, err := RegionNumber(name)
		if err != nil {
			t.Errorf("RegionNumber(%q) err = %v", name, err)
		}
		if got != want {
			t.Errorf("RegionNumber(%q) = %d, want %d", name, got, want)
		}
	}
	if _, err := RegionNumber("Atlantis"); err == nil {
		t.Errorf("RegionNumber(Atlantis) err = nil, want error")
	}
}

func TestValidWorldName(t *testing.T) {
	t.Parallel()
	ok := []string{
		"HADESNET", "ADESTIS", "Test-World_42", "a",
		"ADESTIS RKE2 Lab", "Test Server", "Dune Awakening powered by no-ruto.net",
		"Hadesnet: Offworld", "with/slash", "with.dot", "äöü", strings.Repeat("x", 50),
	}
	bad := []string{"", "with\nnewline", "tab\tchar", "\x00null", "line sep", "para sep", strings.Repeat("x", 51)}
	for _, s := range ok {
		if !ValidWorldName(s) {
			t.Errorf("ValidWorldName(%q) = false, want true", s)
		}
	}
	for _, s := range bad {
		if ValidWorldName(s) {
			t.Errorf("ValidWorldName(%q) = true, want false", s)
		}
	}
}
