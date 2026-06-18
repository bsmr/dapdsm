package gameini

import (
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

func TestAllowlistLookup(t *testing.T) {
	s, ok := Lookup("Sandstorm.Enabled")
	if !ok || s.Section != "ConsoleVariables" || s.Kind != KindInt {
		t.Fatalf("Sandstorm.Enabled lookup: %+v ok=%v", s, ok)
	}
	if _, ok := Lookup("Totally.Unknown"); ok {
		t.Fatal("unknown key must not be in the allowlist")
	}
}

func TestValidateValue(t *testing.T) {
	if err := ValidateValue(KindInt, "1"); err != nil {
		t.Fatalf("int 1: %v", err)
	}
	if err := ValidateValue(KindInt, "x"); err == nil {
		t.Fatal("int x should fail")
	}
	if err := ValidateValue(KindBool, "true"); err != nil {
		t.Fatalf("bool true: %v", err)
	}
	if err := ValidateValue(KindFloat, "1.5"); err != nil {
		t.Fatalf("float 1.5: %v", err)
	}
	if err := ValidateValue(KindBool, "yes"); err == nil {
		t.Fatal("bool \"yes\" should fail")
	}
	if err := ValidateValue(KindString, ""); err == nil {
		t.Fatal("empty string should fail")
	}
}

type iniRunner struct {
	readOut  string
	wrote    []byte
	ranApply bool
}

func (r *iniRunner) Run(ctx context.Context, name string, args ...string) (ssh.Result, error) {
	joined := name + " " + strings.Join(args, " ")
	if strings.Contains(joined, "apply-default-usersettings") {
		r.ranApply = true
	}
	return ssh.Result{Stdout: r.readOut, ExitCode: 0}, nil
}
func (r *iniRunner) RunWithStdin(ctx context.Context, stdin []byte, name string, args ...string) (ssh.Result, error) {
	r.wrote = stdin
	return ssh.Result{ExitCode: 0}, nil
}

func TestSetWritesViaIniconf(t *testing.T) {
	rr := &iniRunner{readOut: "[ConsoleVariables]\nSandstorm.Enabled=0\n"}
	r := &Runner{SSH: &ssh.Client{Runner: rr}}
	if err := r.Set(context.Background(), "h", "Sandstorm.Enabled", "1", false, false); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !strings.Contains(string(rr.wrote), "Sandstorm.Enabled=1") {
		t.Fatalf("written content missing new value: %q", rr.wrote)
	}
	if rr.ranApply {
		t.Fatal("apply must not run without --apply")
	}
}

func TestSetRejectsUnknownKey(t *testing.T) {
	r := &Runner{SSH: &ssh.Client{Runner: &iniRunner{}}}
	if err := r.Set(context.Background(), "h", "Bogus.Key", "1", false, false); err == nil {
		t.Fatal("unknown key must be rejected")
	}
}
