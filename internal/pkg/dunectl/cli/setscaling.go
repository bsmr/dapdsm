package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"go.muehmer.eu/dapdsm/pkg/domain/battlegroup"
	"go.muehmer.eu/dapdsm/pkg/transport/database"
	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

type setScalingDeps struct {
	runner kube.Runner
}

func defaultSetScalingDeps(stderr io.Writer) setScalingDeps {
	return setScalingDeps{runner: &kube.CmdRunner{Stderr: stderr}}
}

func enableSetCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return runSetScaling(ctx, "enable-set", args, stdout, stderr, defaultSetScalingDeps(stderr), false, 1, true)
}

func disableSetCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return runSetScaling(ctx, "disable-set", args, stdout, stderr, defaultSetScalingDeps(stderr), true, 0, false)
}

// runSetScaling implements the common body of enable-set / disable-set.
// allowReplicasFlag controls whether `--replicas N` is exposed (only the
// enable side; for disable the value is fixed at 0).
func runSetScaling(
	ctx context.Context,
	name string,
	args []string,
	stdout, stderr io.Writer,
	deps setScalingDeps,
	dedicatedScaling bool,
	defaultReplicas int,
	allowReplicasFlag bool,
) error {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	ns := fs.String("namespace", "", "BattleGroup namespace (default: first funcom-seabass-*)")
	bg := fs.String("bg-name", "", "BattleGroup name (default: derived from --namespace)")
	replicas := defaultReplicas
	if allowReplicasFlag {
		fs.IntVar(&replicas, "replicas", defaultReplicas, "Number of always-on replicas per matched set")
	}
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: dunectl %s [flags] <map> [<map>...]\n\n", name)
		fs.PrintDefaults()
	}
	if err := fs.Parse(reorderFlagArgs(fs, args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%s: %w: %w", name, ErrUsage, err)
	}

	maps := fs.Args()
	if len(maps) == 0 {
		fmt.Fprintf(stderr, "%s: no map names given\n", name)
		fs.Usage()
		return ErrUsage
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
	// Always-on (dedicatedScaling=false) needs an explicit partitions[N]
	// per map — Funcom-Operator otherwise crashes the pod with "stable
	// server indices disabled". Resolve N from dune.world_partition.
	var partByMap map[string]int
	if !dedicatedScaling {
		creds, err := database.ResolveCreds(ctx, deps.runner, *ns)
		if err != nil {
			return err
		}
		partByMap, err = database.LookupPartitionIDs(ctx, deps.runner, *ns, creds, maps)
		if err != nil {
			return err
		}
	}
	ops, notFound, err := battlegroup.BuildScalingPatches(cr, maps, dedicatedScaling, replicas, partByMap)
	if err != nil {
		return err
	}
	if len(notFound) > 0 {
		return fmt.Errorf("%s: unknown map(s) %s — run 'dunectl list-sets' to see what's available",
			name, strings.Join(notFound, ", "))
	}
	if len(ops) == 0 {
		fmt.Fprintf(stdout, "%s: already in target state, no changes needed\n", name)
		return nil
	}
	payload, err := json.Marshal(ops)
	if err != nil {
		return err
	}
	if err := deps.runner.Patch(ctx, "battlegroup", *bg, *ns, "json", string(payload)); err != nil {
		return err
	}
	verb := "enabled (always-on)"
	if dedicatedScaling {
		verb = "disabled (on-demand)"
	}
	fmt.Fprintf(stdout, "%s [%s], replicas=%d: %s\n", name, verb, replicas, strings.Join(maps, ", "))
	return nil
}
