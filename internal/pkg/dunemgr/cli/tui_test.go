package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// The tui verb must be advertised in usage and routed by Run (not treated as
// an unknown subcommand). We do NOT launch the program here — that needs a TTY
// and is covered by the manual smoke test in Task 7.
func TestUsageMentionsTUI(t *testing.T) {
	var out bytes.Buffer
	if err := Run(context.Background(), []string{"help"}, nil, &out, &out); err != nil {
		t.Fatalf("help: %v", err)
	}
	if !strings.Contains(out.String(), "tui") {
		t.Errorf("usage does not mention the tui command:\n%s", out.String())
	}
}
