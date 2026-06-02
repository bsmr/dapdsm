package command

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

func TestWhisperUnknownPlayerIsUsageError(t *testing.T) {
	var out, errb bytes.Buffer
	// SSH present so discoverDB runs and fails (no such host) → resolvePlayerArg wraps as ErrUsage.
	c := &core.Core{Store: openTestStore(t), SSH: ssh.NewClient()}
	err := whisperCmd(context.Background(), c, []string{"h", "NoSuchName", "hi"}, &out, &errb)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("unknown name should be ErrUsage, got %v", err)
	}
}

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

func TestWhisperAsAndFromMutuallyExclusive(t *testing.T) {
	var out, errb bytes.Buffer
	c := &core.Core{Store: openTestStore(t)}
	err := whisperCmd(context.Background(), c, []string{"h", "ABCD", "hello", "--from", "X", "--as", "GM"}, &out, &errb)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("--from with --as must be ErrUsage, got %v", err)
	}
}

func TestWhisperAsUnknownPersona(t *testing.T) {
	var out, errb bytes.Buffer
	c := &core.Core{Store: openTestStore(t)}
	err := whisperCmd(context.Background(), c, []string{"h", "ABCD", "hello", "--as", "bogus"}, &out, &errb)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("unknown persona must be ErrUsage, got %v", err)
	}
}
