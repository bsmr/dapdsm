package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDispatchUnknownVerb(t *testing.T) {
	var out, errOut bytes.Buffer
	err := Run(context.Background(), []string{"frobnicate"}, strings.NewReader(""), &out, &errOut)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("want ErrUsage, got %v", err)
	}
}

func TestDispatchHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := Run(context.Background(), []string{"help"}, strings.NewReader(""), &out, &errOut); err != nil {
		t.Fatalf("help: %v", err)
	}
	if !strings.Contains(out.String(), "ds-thumper") {
		t.Fatalf("usage missing tool name: %s", out.String())
	}
}

func TestInitRequiresHost(t *testing.T) {
	var out, errOut bytes.Buffer
	err := Run(context.Background(), []string{"init"}, strings.NewReader(""), &out, &errOut)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("want ErrUsage for missing host, got %v", err)
	}
}
