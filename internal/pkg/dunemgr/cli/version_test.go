package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/version"
)

func TestVersionCmdPrintsSharedIdentity(t *testing.T) {
	var out, errb bytes.Buffer
	if err := versionCmd(context.Background(), nil, &out, &errb); err != nil {
		t.Fatalf("versionCmd: %v", err)
	}
	got := out.String()
	if !strings.HasPrefix(got, "dunemgr "+version.Version) {
		t.Errorf("version line = %q, want prefix %q", got, "dunemgr "+version.Version)
	}
	if strings.Contains(got, "v0.0.0 (foundation)") {
		t.Errorf("still prints the old hardcoded string: %q", got)
	}
}
