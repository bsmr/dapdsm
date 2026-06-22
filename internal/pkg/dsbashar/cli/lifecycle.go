package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/exec"

	"go.muehmer.eu/dapdsm/pkg/domain/battlegroup"
	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

// DefaultBattlegroupBin is the location of the Funcom-vendor wrapper
// that orchestrates a BattleGroup's lifecycle (stop / start / restart).
// The wrapper handles operator timing that a plain `kubectl patch` of
// spec.stop would not — so we shell out instead of duplicating it.
const DefaultBattlegroupBin = "/home/dune/.dune/bin/battlegroup"

// vendorRunner runs a single subcommand of the Funcom battlegroup binary
// and streams its output to stdout/stderr. Tests substitute their own.
type vendorRunner func(ctx context.Context, bin, action string, stdout, stderr io.Writer) error

func execVendor(ctx context.Context, bin, action string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, bin, action)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", bin, action, err)
	}
	return nil
}

type lifecycleDeps struct {
	runVendor    vendorRunner
	announceDeps announceDeps
}

func defaultLifecycleDeps() lifecycleDeps {
	return lifecycleDeps{runVendor: execVendor, announceDeps: defaultAnnounceDeps()}
}

func startCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return runLifecycle(ctx, "start", args, stdout, stderr, defaultLifecycleDeps())
}

func stopCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return runLifecycle(ctx, "stop", args, stdout, stderr, defaultLifecycleDeps())
}

func restartCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return runLifecycle(ctx, "restart", args, stdout, stderr, defaultLifecycleDeps())
}

func runLifecycle(ctx context.Context, action string, args []string, stdout, stderr io.Writer, deps lifecycleDeps) error {
	fs := flag.NewFlagSet(action, flag.ContinueOnError)
	fs.SetOutput(stderr)
	bin := fs.String("bg-binary", DefaultBattlegroupBin, "Path to Funcom's battlegroup wrapper (single-node fallback)")
	ns := fs.String("namespace", "", "BattleGroup namespace (K8s-native path; default: first funcom-seabass-*)")
	bg := fs.String("bg-name", "", "BattleGroup name (K8s-native path; default: derived from --namespace)")
	announce := fs.Duration("announce", 0, "Announce a shutdown countdown of this duration, then act (e.g. 5m)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%s: %w: %w", action, ErrUsage, err)
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "%s: unexpected positional argument(s): %v\n", action, fs.Args())
		return ErrUsage
	}
	kind := map[string]string{"restart": "Restart", "stop": "Maintenance"}[action]
	if *announce > 0 && kind == "" {
		fmt.Fprintf(stderr, "%s: --announce is not supported for %s\n", action, action)
		return ErrUsage
	}

	// Multi-node (--jump set): K8s-native CR patch. Single-node: vendor wrapper.
	if resolvedAccess != nil {
		return withAnnounce(ctx, *announce, kind, func(ctx context.Context) error {
			return lifecycleKube(ctx, action, *ns, *bg, stderr)
		}, deps.announceDeps)
	}
	return withAnnounce(ctx, *announce, kind, func(ctx context.Context) error {
		return deps.runVendor(ctx, *bin, action, stdout, stderr)
	}, deps.announceDeps)
}

// lifecycleKube resolves the BG and runs the matching battlegroup primitive.
func lifecycleKube(ctx context.Context, action, ns, bg string, stderr io.Writer) error {
	r := newKubeRunner(stderr)
	if ns == "" {
		found, err := kube.FindBattleGroupNamespace(ctx, r)
		if err != nil {
			return err
		}
		ns = found
	}
	if bg == "" {
		bg = kube.BattleGroupName(ns)
	}
	switch action {
	case "start":
		return battlegroup.Start(ctx, r, ns, bg)
	case "stop":
		return battlegroup.Stop(ctx, r, ns, bg)
	case "restart":
		return battlegroup.Restart(ctx, r, ns, bg)
	default:
		return fmt.Errorf("lifecycleKube: unsupported action %q", action)
	}
}
