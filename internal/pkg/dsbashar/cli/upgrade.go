package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"go.muehmer.eu/dapdsm/pkg/domain/bgorchestrator"
	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

// upgradeCmd runs the status-gated stop→update→start cycle. Multi-node only
// (requires --jump): the orchestrator drives the cluster through the CR.
func upgradeCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	fs.SetOutput(stderr)
	ns := fs.String("namespace", "", "BattleGroup namespace (default: first funcom-seabass-*)")
	bg := fs.String("bg-name", "", "BattleGroup name (default: derived from --namespace)")
	imageTag := fs.String("image-tag", "", "Depot revision tag to reconcile onto the CR (required)")
	poll := fs.Duration("poll", 5*time.Second, "Status poll interval")
	timeout := fs.Duration("timeout", 5*time.Minute, "Per-gate timeout")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("upgrade: %w: %w", ErrUsage, err)
	}
	if resolvedAccess == nil {
		fmt.Fprintln(stderr, "upgrade: multi-node only — set the global --jump flag")
		return ErrUsage
	}
	if *imageTag == "" {
		fmt.Fprintln(stderr, "upgrade: --image-tag is required")
		return ErrUsage
	}

	r := newKubeRunner(stderr)
	n, b := *ns, *bg
	if n == "" {
		found, err := kube.FindBattleGroupNamespace(ctx, r)
		if err != nil {
			return err
		}
		n = found
	}
	if b == "" {
		b = kube.BattleGroupName(n)
	}

	cfg := bgorchestrator.Config{
		Poll:    *poll,
		Timeout: *timeout,
		OnPhase: func(p string) { fmt.Fprintf(stdout, "upgrade: %s\n", p) },
	}
	if err := bgorchestrator.Upgrade(ctx, r, n, b, *imageTag, cfg); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "upgrade: %s/%s upgraded to %s\n", n, b, *imageTag)
	return nil
}
