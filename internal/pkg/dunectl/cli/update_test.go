package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestUpdate_RunsUpdateApplyRestartInOrder(t *testing.T) {
	t.Parallel()
	var calls []recordedVendorCall
	deps := updateDeps{
		runVendor: func(_ context.Context, bin, action string, _, _ io.Writer) error {
			calls = append(calls, recordedVendorCall{bin: bin, action: action})
			return nil
		},
	}
	var stdout, stderr bytes.Buffer
	if err := runUpdate(context.Background(), nil, &stdout, &stderr, deps); err != nil {
		t.Fatalf("err = %v", err)
	}
	want := []string{"update", "apply-default-usersettings", "restart"}
	if len(calls) != len(want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	for i, w := range want {
		if calls[i].action != w {
			t.Errorf("calls[%d].action = %q, want %q", i, calls[i].action, w)
		}
		if calls[i].bin != DefaultBattlegroupBin {
			t.Errorf("calls[%d].bin = %q, want %q", i, calls[i].bin, DefaultBattlegroupBin)
		}
	}
}

func TestUpdate_FromDownloadsFlagSwitchesAction(t *testing.T) {
	t.Parallel()
	var calls []string
	deps := updateDeps{
		runVendor: func(_ context.Context, _, action string, _, _ io.Writer) error {
			calls = append(calls, action)
			return nil
		},
	}
	var stdout, stderr bytes.Buffer
	err := runUpdate(context.Background(), []string{"--from-downloads"}, &stdout, &stderr, deps)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := []string{"update-from-downloads", "apply-default-usersettings", "restart"}
	if len(calls) != len(want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	for i, w := range want {
		if calls[i] != w {
			t.Errorf("calls[%d] = %q, want %q", i, calls[i], w)
		}
	}
}

func TestUpdate_NoRestartFlagSkipsRestart(t *testing.T) {
	t.Parallel()
	var calls []string
	deps := updateDeps{
		runVendor: func(_ context.Context, _, action string, _, _ io.Writer) error {
			calls = append(calls, action)
			return nil
		},
	}
	var stdout, stderr bytes.Buffer
	err := runUpdate(context.Background(), []string{"--no-restart"}, &stdout, &stderr, deps)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := []string{"update", "apply-default-usersettings"}
	if len(calls) != len(want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	for i, w := range want {
		if calls[i] != w {
			t.Errorf("calls[%d] = %q, want %q", i, calls[i], w)
		}
	}
}

func TestUpdate_BgBinaryFlagOverridesDefault(t *testing.T) {
	t.Parallel()
	var got recordedVendorCall
	deps := updateDeps{
		runVendor: func(_ context.Context, bin, action string, _, _ io.Writer) error {
			got = recordedVendorCall{bin: bin, action: action}
			return nil
		},
	}
	var stdout, stderr bytes.Buffer
	err := runUpdate(context.Background(),
		[]string{"--bg-binary", "/tmp/custom-battlegroup", "--no-restart"},
		&stdout, &stderr, deps)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.bin != "/tmp/custom-battlegroup" {
		t.Errorf("bin = %q, want %q", got.bin, "/tmp/custom-battlegroup")
	}
}

func TestUpdate_UpdateErrorAbortsBeforeApply(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("vendor exit 1")
	var calls []string
	deps := updateDeps{
		runVendor: func(_ context.Context, _, action string, _, _ io.Writer) error {
			calls = append(calls, action)
			if action == "update" {
				return sentinel
			}
			return nil
		},
	}
	var stdout, stderr bytes.Buffer
	err := runUpdate(context.Background(), nil, &stdout, &stderr, deps)
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want errors.Is(err, sentinel)", err)
	}
	if len(calls) != 1 || calls[0] != "update" {
		t.Errorf("calls = %v, want [update]", calls)
	}
}

func TestUpdate_ApplyErrorAbortsBeforeRestart(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("vendor exit 1")
	var calls []string
	deps := updateDeps{
		runVendor: func(_ context.Context, _, action string, _, _ io.Writer) error {
			calls = append(calls, action)
			if action == "apply-default-usersettings" {
				return sentinel
			}
			return nil
		},
	}
	var stdout, stderr bytes.Buffer
	err := runUpdate(context.Background(), nil, &stdout, &stderr, deps)
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want errors.Is(err, sentinel)", err)
	}
	want := []string{"update", "apply-default-usersettings"}
	if len(calls) != len(want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}

func TestUpdate_RejectsPositionalArgs(t *testing.T) {
	t.Parallel()
	deps := updateDeps{
		runVendor: func(context.Context, string, string, io.Writer, io.Writer) error { return nil },
	}
	var stdout, stderr bytes.Buffer
	err := runUpdate(context.Background(),
		[]string{"unexpected-positional"},
		&stdout, &stderr, deps)
	if !errors.Is(err, ErrUsage) {
		t.Errorf("err = %v, want errors.Is(err, ErrUsage)", err)
	}
	if !strings.Contains(stderr.String(), "unexpected positional") {
		t.Errorf("stderr = %q, want hint about positional args", stderr.String())
	}
}
