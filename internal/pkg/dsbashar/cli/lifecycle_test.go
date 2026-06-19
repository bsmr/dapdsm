package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
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

func TestLifecycle_AnnounceFlag(t *testing.T) {
	t.Parallel()

	t.Run("restart --announce 5m delegates with correct delay and kind", func(t *testing.T) {
		t.Parallel()
		var ran bool
		var gotDelay time.Duration
		var gotKind string
		deps := lifecycleDeps{
			runVendor: func(_ context.Context, _, _ string, _, _ io.Writer) error {
				ran = true
				return nil
			},
			announceDeps: announceDeps{
				announce: func(_ context.Context, d time.Duration, kind string, action func(context.Context) error) error {
					gotDelay, gotKind = d, kind
					return action(context.Background())
				},
			},
		}
		var stdout, stderr bytes.Buffer
		err := runLifecycle(context.Background(), "restart",
			[]string{"--announce", "5m"},
			&stdout, &stderr, deps)
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if !ran {
			t.Error("vendor action was not called")
		}
		if gotDelay != 5*time.Minute {
			t.Errorf("delay = %v, want 5m", gotDelay)
		}
		if gotKind != "Restart" {
			t.Errorf("kind = %q, want Restart", gotKind)
		}
	})

	t.Run("start --announce 5m returns ErrUsage", func(t *testing.T) {
		t.Parallel()
		deps := lifecycleDeps{
			runVendor: func(context.Context, string, string, io.Writer, io.Writer) error { return nil },
		}
		var stdout, stderr bytes.Buffer
		err := runLifecycle(context.Background(), "start",
			[]string{"--announce", "5m"},
			&stdout, &stderr, deps)
		if !errors.Is(err, ErrUsage) {
			t.Errorf("err = %v, want ErrUsage", err)
		}
	})
}
