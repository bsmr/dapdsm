package cli

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"slices"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dsbashar/config"
)

func join(parts []string) string { return strings.Join(parts, " ") }

func TestBringup_FreshCluster_RunsSetupThenReconcile(t *testing.T) {
	var steps []string
	deps := bringupDeps{
		listBGs: func(context.Context) ([]string, error) { return nil, nil }, // fresh
		promote: func(_ context.Context, _ config.Config, _, _ []byte) error {
			steps = append(steps, "promote")
			return nil
		},
		verifyStorageClass: func(context.Context) error { return nil },
		runSetup:           func(context.Context, config.Config, []byte) error { steps = append(steps, "setup"); return nil },
		loadImg:            func(context.Context, config.Target) error { steps = append(steps, "image"); return nil },
		reconcileImageTags: func(context.Context, config.Target) error { return nil },
		initDB:             func(context.Context, io.Writer, io.Writer) error { steps = append(steps, "init-db"); return nil },
		reconcile:          func(context.Context, io.Writer, io.Writer) error { steps = append(steps, "reconcile"); return nil },
		loadCluster:        func(context.Context) (config.Config, bool, error) { return config.Config{}, false, nil },
		readToken:          func(string) ([]byte, error) { return []byte("tok"), nil },
	}
	in := resolveInput{
		Flags:        config.Override{WorldName: "Arrakis", WorldRegion: "Europe", ServerDisplayName: "S"},
		FLSTokenFlag: "/t/fls", BGNameFlag: "Arrakis",
	}
	var out bytes.Buffer
	if err := runBringup(context.Background(), in, bufio.NewReader(nil), &out, io.Discard, deps); err != nil {
		t.Fatalf("runBringup: %v", err)
	}
	got := join(steps)
	if got != "promote setup image init-db reconcile" {
		t.Fatalf("step order = %q", got)
	}
}

func TestBringup_ExistingBG_SkipsSetup(t *testing.T) {
	var steps []string
	deps := bringupDeps{
		listBGs: func(context.Context) ([]string, error) { return []string{"funcom-seabass-abc"}, nil },
		promote: func(_ context.Context, _ config.Config, _, _ []byte) error {
			steps = append(steps, "promote")
			return nil
		},
		verifyStorageClass: func(context.Context) error { return nil },
		runSetup:           func(context.Context, config.Config, []byte) error { steps = append(steps, "setup"); return nil },
		loadImg:            func(context.Context, config.Target) error { steps = append(steps, "image"); return nil },
		reconcileImageTags: func(context.Context, config.Target) error { return nil },
		initDB:             func(context.Context, io.Writer, io.Writer) error { steps = append(steps, "init-db"); return nil },
		reconcile:          func(context.Context, io.Writer, io.Writer) error { steps = append(steps, "reconcile"); return nil },
		loadCluster:        func(context.Context) (config.Config, bool, error) { return config.Config{}, false, nil },
		readToken:          func(string) ([]byte, error) { return []byte("tok"), nil },
	}
	in := resolveInput{Flags: config.Override{WorldName: "Arrakis", WorldRegion: "Europe", ServerDisplayName: "S"},
		FLSTokenFlag: "/t/fls", BGNameFlag: "Arrakis"}
	if err := runBringup(context.Background(), in, bufio.NewReader(nil), io.Discard, io.Discard, deps); err != nil {
		t.Fatalf("runBringup: %v", err)
	}
	for _, s := range steps {
		if s == "setup" {
			t.Fatalf("setup must be skipped when a BG already exists: %v", steps)
		}
	}
}

func TestRunBringup_VerifyAndReconcileOrder(t *testing.T) {
	var calls []string
	rec := func(name string) func(context.Context) error {
		return func(context.Context) error { calls = append(calls, name); return nil }
	}
	d := bringupDeps{
		loadCluster:        func(context.Context) (config.Config, bool, error) { return config.Config{}, false, nil },
		verifyStorageClass: rec("verify-sc"),
		listBGs:            func(context.Context) ([]string, error) { calls = append(calls, "list"); return nil, nil },
		promote:            func(context.Context, config.Config, []byte, []byte) error { calls = append(calls, "promote"); return nil },
		runSetup:           func(context.Context, config.Config, []byte) error { calls = append(calls, "setup"); return nil },
		loadImg:            func(context.Context, config.Target) error { calls = append(calls, "load"); return nil },
		reconcileImageTags: func(_ context.Context, _ config.Target) error { calls = append(calls, "reconcile-tags"); return nil },
		initDB:             func(context.Context, io.Writer, io.Writer) error { calls = append(calls, "initdb"); return nil },
		reconcile:          func(context.Context, io.Writer, io.Writer) error { calls = append(calls, "reconcile"); return nil },
		readToken:          func(string) ([]byte, error) { return []byte("t"), nil },
	}
	in := resolveInput{Flags: config.Override{WorldName: "BG", WorldRegion: "Europe"}, NoInput: true, FLSTokenFlag: "/tmp/t", BGNameFlag: "BG"}
	if err := runBringup(context.Background(), in, bufio.NewReader(strings.NewReader("")), io.Discard, io.Discard, d); err != nil {
		t.Fatalf("runBringup: %v", err)
	}
	want := []string{"verify-sc", "promote", "list", "setup", "load", "reconcile-tags", "initdb", "reconcile"}
	if !slices.Equal(calls, want) {
		t.Errorf("call order = %v, want %v", calls, want)
	}
}
