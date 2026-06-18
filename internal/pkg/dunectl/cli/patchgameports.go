package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"go.muehmer.eu/dapdsm/pkg/domain/battlegroup"
	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

type patchGamePortsDeps struct {
	runner kube.Runner
}

func patchGamePortsCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return runPatchGamePorts(ctx, args, stdout, stderr, patchGamePortsDeps{
		runner: &kube.CmdRunner{Stderr: stderr},
	})
}

// runPatchGamePorts adds (or replaces) the per-set CLI args
//
//	-ini:engine:[URL]:Port=<gameBase>
//	-ini:engine:[URL]:IGWPort=<igwBase>
//
// on every game-server set in the live BattleGroup CR. Needed when two
// BattleGroups share a public IP behind a vCD-style edge — without it
// both servers try to bind UDP 7777/7888 and the second one's traffic
// hits the wrong VM after DNAT.
//
// Idempotent: a set already carrying the target values produces no op.
func runPatchGamePorts(ctx context.Context, args []string, stdout, stderr io.Writer, deps patchGamePortsDeps) error {
	fs := flag.NewFlagSet("patch-game-ports", flag.ContinueOnError)
	fs.SetOutput(stderr)
	ns := fs.String("namespace", "", "BattleGroup namespace (default: first funcom-seabass-*)")
	bg := fs.String("bg-name", "", "BattleGroup name (default: derived from --namespace)")
	gameBase := fs.Int("game-base", 0, "Game UDP base port (e.g. 7877). Required.")
	igwBase := fs.Int("igw-base", 0, "IGW UDP base port (e.g. 7988). Required.")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: dunectl patch-game-ports --game-base N --igw-base M\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("patch-game-ports: %w: %w", ErrUsage, err)
	}
	if *gameBase <= 0 {
		return fmt.Errorf("patch-game-ports: --game-base N required (got %d): %w", *gameBase, ErrUsage)
	}
	if *igwBase <= 0 {
		return fmt.Errorf("patch-game-ports: --igw-base M required (got %d): %w", *igwBase, ErrUsage)
	}

	if *ns == "" {
		found, err := kube.FindBattleGroupNamespace(ctx, deps.runner)
		if err != nil {
			return err
		}
		*ns = found
	}
	if *bg == "" {
		*bg = kube.BattleGroupName(*ns)
	}

	cr, err := deps.runner.Get(ctx, "battlegroup", *bg, "-n", *ns, "-o", "json")
	if err != nil {
		return err
	}
	ops, err := battlegroup.BuildPortPatches(cr, *gameBase, *igwBase)
	if err != nil {
		return err
	}
	if len(ops) == 0 {
		fmt.Fprintf(stdout, "patch-game-ports: already in target state, no changes needed\n")
		return nil
	}
	payload, err := json.Marshal(ops)
	if err != nil {
		return err
	}
	if err := deps.runner.Patch(ctx, "battlegroup", *bg, *ns, "json", string(payload)); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "patch-game-ports: applied %d op(s) (game-base=%d, igw-base=%d)\n",
		len(ops), *gameBase, *igwBase)
	return nil
}
