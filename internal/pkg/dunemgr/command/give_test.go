package command

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

func TestGiveUnknownPlayerIsUsageError(t *testing.T) {
	var out, errb bytes.Buffer
	// SSH present so discoverDB runs and fails (no such host) → resolvePlayerArg wraps as ErrUsage.
	c := &core.Core{Store: openTestStore(t), SSH: ssh.NewClient()}
	// "skillpoints NoSuchName 1" — valid sub + amount so resolution is exercised
	err := giveCmd(context.Background(), c, []string{"h", "skillpoints", "NoSuchName", "1"}, &out, &errb)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("unknown name should be ErrUsage, got %v", err)
	}
}

func TestGiveKnown(t *testing.T) {
	if !Known("give") {
		t.Fatal("give should be a known verb")
	}
}

func TestGiveUsageErrors(t *testing.T) {
	cases := [][]string{
		{"give"},
		{"give", "h"},
		{"give", "h", "bogus", "FLS"},
		{"give", "h", "currency", "FLS"},
		{"give", "h", "item", "FLS", "Item_X"},
		{"give", "h", "skillpoints", "FLS"},
	}
	for _, argv := range cases {
		var out, errb bytes.Buffer
		err := Dispatch(context.Background(), &core.Core{}, argv, &out, &errb)
		if !errors.Is(err, ErrUsage) {
			t.Fatalf("argv %v: want ErrUsage, got %v", argv, err)
		}
	}
}

func TestGiveSpecRegistered(t *testing.T) {
	s, ok := SpecFor("give")
	if !ok {
		t.Fatal("give spec missing")
	}
	if s.Verb != "give" || len(s.Args) < 2 {
		t.Fatalf("give spec malformed: %+v", s)
	}
}

func TestGiveSpecHasXPVerbs(t *testing.T) {
	s, ok := SpecFor("give")
	if !ok {
		t.Fatal("give spec missing")
	}
	var opts []string
	for _, a := range s.Args {
		if a.name == "sub" {
			opts = a.options
		}
	}
	for _, want := range []string{"currency", "item", "skillpoints", "xp", "charxp"} {
		found := false
		for _, o := range opts {
			if o == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("give sub options missing %q: %v", want, opts)
		}
	}
}
