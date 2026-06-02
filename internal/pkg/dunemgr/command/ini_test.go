package command

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
)

func TestIniKnownAndUsage(t *testing.T) {
	if !Known("ini") {
		t.Fatal("ini should be known")
	}
	for _, argv := range [][]string{
		{"ini"}, {"ini", "h"}, {"ini", "h", "bogus"},
		{"ini", "h", "get"}, {"ini", "h", "set", "k"},
	} {
		var o, e bytes.Buffer
		if err := Dispatch(context.Background(), &core.Core{}, argv, &o, &e); !errors.Is(err, ErrUsage) {
			t.Fatalf("argv %v: want ErrUsage, got %v", argv, err)
		}
	}
}

func TestIniSpecKeySlotIsCatalog(t *testing.T) {
	s, ok := SpecFor("ini")
	if !ok {
		t.Fatal("no ini spec")
	}
	if s.Args[2].kind != argCatalog || s.Args[2].catalogKey != "ini" {
		t.Fatalf("ini key slot should be argCatalog 'ini': %+v", s.Args[2])
	}
}
