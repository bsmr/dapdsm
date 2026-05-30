package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type recordedVendorCall struct {
	bin, action string
}

func TestLifecycle_DispatchesActionToVendorBinary(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"start", "stop", "restart"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			var got recordedVendorCall
			deps := lifecycleDeps{
				runVendor: func(_ context.Context, bin, action string, stdout, _ io.Writer) error {
					got = recordedVendorCall{bin: bin, action: action}
					return nil
				},
			}
			var stdout, stderr bytes.Buffer
			if err := runLifecycle(context.Background(), action, nil, &stdout, &stderr, deps); err != nil {
				t.Fatalf("err = %v", err)
			}
			if got.action != action {
				t.Errorf("action = %q, want %q", got.action, action)
			}
			if got.bin != DefaultBattlegroupBin {
				t.Errorf("bin = %q, want %q", got.bin, DefaultBattlegroupBin)
			}
		})
	}
}

func TestLifecycle_BgBinaryFlagOverridesDefault(t *testing.T) {
	t.Parallel()
	var got recordedVendorCall
	deps := lifecycleDeps{
		runVendor: func(_ context.Context, bin, action string, _, _ io.Writer) error {
			got = recordedVendorCall{bin: bin, action: action}
			return nil
		},
	}
	var stdout, stderr bytes.Buffer
	err := runLifecycle(context.Background(), "start",
		[]string{"--bg-binary", "/tmp/custom-battlegroup"},
		&stdout, &stderr, deps)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.bin != "/tmp/custom-battlegroup" {
		t.Errorf("bin = %q, want %q", got.bin, "/tmp/custom-battlegroup")
	}
}

func TestLifecycle_PropagatesVendorError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("vendor exit 1")
	deps := lifecycleDeps{
		runVendor: func(context.Context, string, string, io.Writer, io.Writer) error {
			return sentinel
		},
	}
	var stdout, stderr bytes.Buffer
	err := runLifecycle(context.Background(), "stop", nil, &stdout, &stderr, deps)
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want errors.Is(err, sentinel)", err)
	}
}

func TestLifecycle_RejectsPositionalArgs(t *testing.T) {
	t.Parallel()
	deps := lifecycleDeps{runVendor: func(context.Context, string, string, io.Writer, io.Writer) error { return nil }}
	var stdout, stderr bytes.Buffer
	err := runLifecycle(context.Background(), "start",
		[]string{"unexpected-positional"},
		&stdout, &stderr, deps)
	if !errors.Is(err, ErrUsage) {
		t.Errorf("err = %v, want errors.Is(err, ErrUsage)", err)
	}
	if !strings.Contains(stderr.String(), "unexpected positional") {
		t.Errorf("stderr = %q, want hint about positional args", stderr.String())
	}
}
