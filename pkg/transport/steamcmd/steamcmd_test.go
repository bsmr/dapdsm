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
