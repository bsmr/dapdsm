package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRun_NoArgs_ReturnsErrUsageAndPrintsUsageToStderr(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), nil, stubExecer{}, nil, &stdout, &stderr)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("Run() err = %v, want errors.Is(err, ErrUsage)", err)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("stderr = %q, want it to contain \"Usage:\"", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func TestRun_HelpFlavors_PrintUsageToStdout(t *testing.T) {
	t.Parallel()
	for _, arg := range []string{"help", "-h", "--help"} {
		t.Run(arg, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			if err := Run(context.Background(), []string{arg}, stubExecer{}, nil, &stdout, &stderr); err != nil {
				t.Fatalf("Run(%q) err = %v, want nil", arg, err)
			}
			if !strings.Contains(stdout.String(), "Usage:") {
				t.Errorf("stdout = %q, want it to contain \"Usage:\"", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRun_UnknownSubcommand_ReturnsErrUsage(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), []string{"frobnicate"}, stubExecer{}, nil, &stdout, &stderr)
	if !errors.Is(err, ErrUsage) {
		t.Errorf("Run() err = %v, want errors.Is(err, ErrUsage)", err)
	}
}
