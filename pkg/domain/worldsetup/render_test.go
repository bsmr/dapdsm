package worldsetup

import (
	"strings"
	"testing"
)

func TestRenderTemplate_AllTokens(t *testing.T) {
	tmpl := "name={WORLD_NAME} uniq={WORLD_UNIQUE_NAME} tag={WORLD_IMAGE_TAG} rmq={RMQ_SECRET}"
	out := renderTemplate(tmpl, map[string]string{
		"WORLD_NAME":       "MyBG",
		"WORLD_UNIQUE_NAME": "sh-abc-aaaaaa",
		"WORLD_IMAGE_TAG":  PlaceholderImageTag,
		"RMQ_SECRET":       "ab/cd==", // base64 may contain '/'
	})
	want := "name=MyBG uniq=sh-abc-aaaaaa tag=0-0-shipping rmq=ab/cd=="
	if out != want {
		t.Errorf("renderTemplate =\n%q\nwant\n%q", out, want)
	}
}

func TestYAMLQuote(t *testing.T) {
	cases := map[string]string{
		"ADESTIS RKE2 Lab":   `"ADESTIS RKE2 Lab"`,   // spaces
		"Hadesnet: Offworld": `"Hadesnet: Offworld"`, // colon-space (the bare-scalar breaker)
		"no-ruto.net":        `"no-ruto.net"`,        // dot + hyphen
		`a "b" \c`:           `"a \"b\" \\c"`,         // escape quotes and backslash
	}
	for in, want := range cases {
		if got := yamlQuote(in); got != want {
			t.Errorf("yamlQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRenderTemplate_NoLeftoverTokens(t *testing.T) {
	out := renderTemplate("{WORLD_NAME}/{FLS_SECRET}", map[string]string{
		"WORLD_NAME": "x", "FLS_SECRET": "y",
	})
	if strings.Contains(out, "{") || strings.Contains(out, "}") {
		t.Errorf("unsubstituted token left: %q", out)
	}
}
