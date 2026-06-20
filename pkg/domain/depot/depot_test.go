package depot

import (
	"context"
	"strings"
	"testing"
)

// fakeRunner returns canned stdout per command name; records calls.
type fakeRunner struct {
	calls  [][]string
	catOut string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	if name == "cat" {
		return f.catOut, nil
	}
	return "", nil
}

func TestAcquire(t *testing.T) {
	f := &fakeRunner{catOut: "v1.5.0\n"}
	res, err := Acquire(context.Background(), f, 4754530, "/home/dune/depot/prod")
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if res.Version != "v1.5.0" {
		t.Errorf("Version = %q, want v1.5.0 (trimmed)", res.Version)
	}
	if res.OperatorsDir != "/home/dune/depot/prod/images/operators" {
		t.Errorf("OperatorsDir = %q", res.OperatorsDir)
	}
	if res.PrerequisitesDir != "/home/dune/depot/prod/images/prerequisites" {
		t.Errorf("PrerequisitesDir = %q", res.PrerequisitesDir)
	}
	if res.BattlegroupDir != "/home/dune/depot/prod/images/battlegroup" {
		t.Errorf("BattlegroupDir = %q", res.BattlegroupDir)
	}
	// Ordered: EnsureInstalled (sudo bash -c), AppUpdate (steamcmd ...), cat version.txt.
	joined := ""
	for _, c := range f.calls {
		joined += strings.Join(c, " ") + "\n"
	}
	for _, want := range []string{
		"sudo bash -c",
		"steamcmd +force_install_dir /home/dune/depot/prod",
		"+app_update 4754530",
		"cat /home/dune/depot/prod/images/operators/version.txt",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in calls:\n%s", want, joined)
		}
	}
}

func TestAcquire_AppUpdateError(t *testing.T) {
	fe := &errRunner{failOn: "steamcmd"}
	if _, err := Acquire(context.Background(), fe, 4754530, "/d"); err == nil {
		t.Fatal("want error when app_update fails")
	}
}

// errRunner fails when the command name matches failOn.
type errRunner struct{ failOn string }

func (e *errRunner) Run(_ context.Context, name string, _ ...string) (string, error) {
	if name == e.failOn {
		return "", context.DeadlineExceeded
	}
	return "", nil
}
