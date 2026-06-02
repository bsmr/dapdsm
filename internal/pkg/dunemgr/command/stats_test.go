package command

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
)

func TestStatsKnownAndUsage(t *testing.T) {
	if !Known("stats") {
		t.Fatal("stats should be known")
	}
	var o, e bytes.Buffer
	if err := Dispatch(context.Background(), &core.Core{}, []string{"stats"}, &o, &e); !errors.Is(err, ErrUsage) {
		t.Fatalf("stats without host: want ErrUsage, got %v", err)
	}
}

func TestFormatBytes(t *testing.T) {
	if got := formatBytes(1536); got != "1.5 KiB" {
		t.Fatalf("formatBytes(1536)=%q", got)
	}
	if got := formatBytes(0); got != "0 B" {
		t.Fatalf("formatBytes(0)=%q", got)
	}
}
