package command

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
)

func TestWhisperKnownAndUsage(t *testing.T) {
	if !Known("whisper") {
		t.Fatal("whisper should be known")
	}
	for _, argv := range [][]string{
		{"whisper"}, {"whisper", "h"}, {"whisper", "h", "fls"},
	} {
		var o, e bytes.Buffer
		if err := Dispatch(context.Background(), &core.Core{}, argv, &o, &e); !errors.Is(err, ErrUsage) {
			t.Fatalf("argv %v: want ErrUsage, got %v", argv, err)
		}
	}
}

func TestWhisperEmptyMessage(t *testing.T) {
	var o, e bytes.Buffer
	// host + fls present, but the only remaining token is a flag → empty message.
	err := Dispatch(context.Background(), &core.Core{}, []string{"whisper", "h", "fls", "--force"}, &o, &e)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("empty message should be ErrUsage, got %v", err)
	}
}
