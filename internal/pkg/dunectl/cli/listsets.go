package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"go.muehmer.eu/dapdsm/internal/pkg/battlegroup"
	"go.muehmer.eu/dapdsm/internal/pkg/kube"
)

type listSetsDeps struct {
	runner kube.Runner
}

func defaultListSetsDeps(stderr io.Writer) listSetsDeps {
	return listSetsDeps{runner: &kube.CmdRunner{Stderr: stderr}}
}

func listSetsCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return listSets(ctx, args, stdout, stderr, defaultListSetsDeps(stderr))
}

func listSets(ctx context.Context, args []string, stdout, stderr io.Writer, deps listSetsDeps) error {
	fs := flag.NewFlagSet("list-sets", flag.ContinueOnError)
	fs.SetOutput(stderr)
	ns := fs.String("namespace", "", "BattleGroup namespace (default: first funcom-seabass-*)")
	bg := fs.String("bg-name", "", "BattleGroup name (default: derived from --namespace)")
	asJSON := fs.Bool("json", false, "Emit JSON instead of a table")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("list-sets: %w: %w", ErrUsage, err)
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
	sets, err := battlegroup.ListSets(cr)
	if err != nil {
		return err
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(sets)
	}

	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "MAP\tSCALING\tREPLICAS\tPARTITIONS")
	for _, s := range sets {
		scaling := "always-on"
		if s.DedicatedScaling {
			scaling = "on-demand"
		}
		repl := "null"
		if s.Replicas != nil {
			repl = strconv.Itoa(*s.Replicas)
		}
		partitions := "-"
		if len(s.Partitions) > 0 {
			parts := make([]string, len(s.Partitions))
			for i, p := range s.Partitions {
				parts[i] = strconv.Itoa(p)
			}
			partitions = strings.Join(parts, ",")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.Map, scaling, repl, partitions)
	}
	return tw.Flush()
}
