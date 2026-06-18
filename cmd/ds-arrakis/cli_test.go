package main

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestRun_UnknownSubcommand(t *testing.T) {
	err := run(context.Background(), []string{"frobnicate"}, &bytes.Buffer{}, &bytes.Buffer{})
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("want ErrUsage, got %v", err)
	}
}

func TestRun_NoArgs(t *testing.T) {
	err := run(context.Background(), nil, &bytes.Buffer{}, &bytes.Buffer{})
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("want ErrUsage, got %v", err)
	}
}
