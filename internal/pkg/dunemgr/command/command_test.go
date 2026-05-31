package command

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestDispatchUnknownVerbReturnsErrUsage(t *testing.T) {
	var out, errb bytes.Buffer
	err := Dispatch(context.Background(), nil, []string{"frobnicate"}, &out, &errb)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("Dispatch unknown verb: err = %v, want ErrUsage", err)
	}
}

func TestDispatchNoArgsReturnsErrUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if err := Dispatch(context.Background(), nil, nil, &out, &errb); !errors.Is(err, ErrUsage) {
		t.Fatalf("Dispatch no args: err = %v, want ErrUsage", err)
	}
}

func TestKnownMatchesRegisteredVerbs(t *testing.T) {
	for _, v := range []string{"host", "lifecycle", "broadcast", "db", "backup", "shutdown"} {
		if !Known(v) {
			t.Errorf("Known(%q) = false, want true", v)
		}
	}
	for _, v := range []string{"frobnicate", "version", "regen-token", ""} {
		if Known(v) {
			t.Errorf("Known(%q) = true, want false", v)
		}
	}
}
