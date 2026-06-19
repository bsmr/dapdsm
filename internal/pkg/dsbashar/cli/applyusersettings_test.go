package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestApplyUserSettings_RunsApplyOnlyByDefault(t *testing.T) {
	t.Parallel()
	var calls []recordedVendorCall
	deps := applyUserSettingsDeps{
		runVendor: func(_ context.Context, bin, action string, _, _ io.Writer) error {
			calls = append(calls, recordedVendorCall{bin: bin, action: action})
			return nil
		},
	}
	var stdout, stderr bytes.Buffer
	if err := applyUserSettings(context.Background(), nil, &stdout, &stderr, deps); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %v, want exactly one", calls)
	}
	if calls[0].action != "apply-default-usersettings" {
		t.Errorf("action = %q, want apply-default-usersettings", calls[0].action)
	}
	if calls[0].bin != DefaultBattlegroupBin {
		t.Errorf("bin = %q, want %q", calls[0].bin, DefaultBattlegroupBin)
	}
}

func TestApplyUserSettings_RestartFlagAddsRestartCall(t *testing.T) {
	t.Parallel()
	var calls []recordedVendorCall
	deps := applyUserSettingsDeps{
		runVendor: func(_ context.Context, bin, action string, _, _ io.Writer) error {
			calls = append(calls, recordedVendorCall{bin: bin, action: action})
			return nil
		},
	}
	var stdout, stderr bytes.Buffer
	if err := applyUserSettings(context.Background(), []string{"--restart"}, &stdout, &stderr, deps); err != nil {
		t.Fatalf("err = %v", err)
	}
	want := []string{"apply-default-usersettings", "restart"}
	if len(calls) != len(want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	for i, w := range want {
		if calls[i].action != w {
			t.Errorf("calls[%d].action = %q, want %q", i, calls[i].action, w)
		}
	}
}

func TestApplyUserSettings_BgBinaryFlagOverridesDefault(t *testing.T) {
	t.Parallel()
	var got recordedVendorCall
	deps := applyUserSettingsDeps{
		runVendor: func(_ context.Context, bin, action string, _, _ io.Writer) error {
			got = recordedVendorCall{bin: bin, action: action}
			return nil
		},
	}
	var stdout, stderr bytes.Buffer
	err := applyUserSettings(context.Background(),
		[]string{"--bg-binary", "/tmp/custom-battlegroup"},
		&stdout, &stderr, deps)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.bin != "/tmp/custom-battlegroup" {
		t.Errorf("bin = %q, want %q", got.bin, "/tmp/custom-battlegroup")
	}
}

func TestApplyUserSettings_ApplyErrorSkipsRestart(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("vendor exit 1")
	var calls []string
	deps := applyUserSettingsDeps{
		runVendor: func(_ context.Context, _, action string, _, _ io.Writer) error {
			calls = append(calls, action)
			return sentinel
		},
	}
	var stdout, stderr bytes.Buffer
	err := applyUserSettings(context.Background(), []string{"--restart"}, &stdout, &stderr, deps)
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want errors.Is(err, sentinel)", err)
	}
	if len(calls) != 1 || calls[0] != "apply-default-usersettings" {
		t.Errorf("calls = %v, want only apply-default-usersettings", calls)
	}
}

func TestApplyUserSettings_RejectsPositionalArgs(t *testing.T) {
	t.Parallel()
	deps := applyUserSettingsDeps{
		runVendor: func(context.Context, string, string, io.Writer, io.Writer) error { return nil },
	}
	var stdout, stderr bytes.Buffer
	err := applyUserSettings(context.Background(),
		[]string{"unexpected-positional"},
		&stdout, &stderr, deps)
	if !errors.Is(err, ErrUsage) {
		t.Errorf("err = %v, want errors.Is(err, ErrUsage)", err)
	}
	if !strings.Contains(stderr.String(), "unexpected positional") {
		t.Errorf("stderr = %q, want hint about positional args", stderr.String())
	}
}
