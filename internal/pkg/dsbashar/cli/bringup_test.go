package cli

import (
	"bufio"
	"bytes"
	"context"
	"io"
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
		runSetup:    func(context.Context, config.Config, string) error { steps = append(steps, "setup"); return nil },
		loadImg:     func(context.Context) error { steps = append(steps, "image"); return nil },
		initDB:      func(context.Context, io.Writer, io.Writer) error { steps = append(steps, "init-db"); return nil },
		reconcile:   func(context.Context, io.Writer, io.Writer) error { steps = append(steps, "reconcile"); return nil },
		loadCluster: func(context.Context) (config.Config, bool, error) { return config.Config{}, false, nil },
		readToken:   func(string) ([]byte, error) { return []byte("tok"), nil },
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
		runSetup:    func(context.Context, config.Config, string) error { steps = append(steps, "setup"); return nil },
		loadImg:     func(context.Context) error { steps = append(steps, "image"); return nil },
		initDB:      func(context.Context, io.Writer, io.Writer) error { steps = append(steps, "init-db"); return nil },
		reconcile:   func(context.Context, io.Writer, io.Writer) error { steps = append(steps, "reconcile"); return nil },
		loadCluster: func(context.Context) (config.Config, bool, error) { return config.Config{}, false, nil },
		readToken:   func(string) ([]byte, error) { return []byte("tok"), nil },
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
