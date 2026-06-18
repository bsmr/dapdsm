package kubedist

import (
	"context"
	"strings"
	"testing"
)

func TestFakeRunnerRecordsCalls(t *testing.T) {
	f := &FakeRunner{Outputs: map[string]string{"echo": "hi\n"}}
	out, err := f.Run(context.Background(), "echo", "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out) != "hi" {
		t.Fatalf("got %q", out)
	}
	if len(f.Calls) != 1 || f.Calls[0][0] != "echo" {
		t.Fatalf("calls not recorded: %v", f.Calls)
	}
}

func TestNewRunnerRunsRealCommand(t *testing.T) {
	out, err := NewRunner().Run(context.Background(), "true")
	if err != nil {
		t.Fatalf("true failed: %v", err)
	}
	_ = out
}
