package steamcmd

import (
	"context"
	"strings"
	"testing"
)

type fakeRunner struct{ calls [][]string }

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	return "", nil
}

func TestEnsureInstalled_RunsEmbeddedScriptViaSudoBash(t *testing.T) {
	f := &fakeRunner{}
	if err := EnsureInstalled(context.Background(), f); err != nil {
		t.Fatalf("EnsureInstalled: %v", err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(f.calls))
	}
	c := f.calls[0]
	if len(c) != 4 || c[0] != "sudo" || c[1] != "bash" || c[2] != "-c" {
		t.Fatalf("call = %v, want [sudo bash -c <script>]", c[:min(4, len(c))])
	}
	script := c[3]
	// The embedded script must carry the distro-aware, trixie-correct logic.
	for _, want := range []string{
		"dpkg --add-architecture i386",
		"steamcmd-non-free.sources",
		"Components: non-free contrib",
		"Suites: ${VERSION_CODENAME}",
		"steam steam/question select I AGREE",
		"ln -sfn /usr/games/steamcmd /usr/local/bin/steamcmd",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("embedded script missing %q", want)
		}
	}
}

func TestAppUpdate_BuildsSteamcmdArgs(t *testing.T) {
	f := &fakeRunner{}
	if err := AppUpdate(context.Background(), f, 4754530, "/home/dune/.dune/download"); err != nil {
		t.Fatal(err)
	}
	joined := f.calls[0]
	got := strings.Join(joined, " ")
	for _, want := range []string{"steamcmd", "+login anonymous", "+app_update 4754530", "/home/dune/.dune/download", "+quit"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in: %s", want, got)
		}
	}
}
