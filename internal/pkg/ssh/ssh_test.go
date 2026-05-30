package ssh

import (
	"context"
	"testing"
)

func TestResultZeroValue(t *testing.T) {
	var r Result
	if r.ExitCode != 0 {
		t.Errorf("zero-value Result.ExitCode = %d, want 0", r.ExitCode)
	}
	if r.Stdout != "" || r.Stderr != "" {
		t.Errorf("zero-value Result has non-empty Stdout/Stderr")
	}
}

func TestRunnerInterface(t *testing.T) {
	// Compile-time check: realRunner satisfies Runner
	var _ Runner = (*realRunner)(nil)
	// Compile-time check: NewRunner returns Runner
	_ = NewRunner()
	// Smoke: NewRunner with context — does not panic
	_, err := NewRunner().Run(context.Background(), "echo", "hi")
	if err != nil {
		t.Errorf("real runner echo failed: %v", err)
	}
}
