package command

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
)

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
