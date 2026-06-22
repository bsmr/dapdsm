package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"go.muehmer.eu/dapdsm/pkg/domain/battlegroup"
	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

type statusDeps struct {
	runner kube.Runner
}

func defaultStatusDeps(stderr io.Writer) statusDeps {
	return statusDeps{runner: newKubeRunner(stderr)}
}

func statusCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return runStatus(ctx, args, stdout, stderr, defaultStatusDeps(stderr))
}

// runStatus reads the BattleGroup CR and prints its observed .status: the
// overall/serverGroup/db/director phases plus the per-map server table. This is
// the K8s-native equivalent of Funcom's single-node `battlegroup status` (the
// vendor wrapper is absent in the multi-node model).
func runStatus(ctx context.Context, args []string, stdout, stderr io.Writer, deps statusDeps) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	ns := fs.String("namespace", "", "BattleGroup namespace (default: first funcom-seabass-*)")
	bg := fs.String("bg-name", "", "BattleGroup name (default: derived from --namespace)")
	asJSON := fs.Bool("json", false, "Emit JSON instead of a table")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("status: %w: %w", ErrUsage, err)
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
	st, err := battlegroup.ParseStatus(cr)
	if err != nil {
		return err
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(st)
	}

	fmt.Fprintf(stdout, "battlegroup: %s  (namespace %s)\n", *bg, *ns)
	fmt.Fprintf(stdout, "phase:       %s   serverGroup: %s   size: %d\n",
		orNone(st.Phase), orNone(st.ServerGroupPhase), st.Size)
	fmt.Fprintf(stdout, "database:    %s   director: %s\n",
		orNone(st.DBPhase), orNone(st.DirectorPhase))
	if !st.StartedAt.IsZero() {
		fmt.Fprintf(stdout, "started:     %s\n", st.StartedAt.Format("2006-01-02 15:04:05 MST"))
	}

	if len(st.Servers) == 0 {
		fmt.Fprintln(stdout, "\nno servers (BattleGroup stopped or no map enabled)")
		return nil
	}
	fmt.Fprintln(stdout)
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "MAP\tPHASE\tREADY\tGAME\tIGW\tRESTARTS\tLAST EXIT")
	for _, s := range st.Servers {
		fmt.Fprintf(tw, "%s\t%s\t%t\t%s\t%s\t%d\t%s\n",
			s.Map, s.Phase, s.Ready, port(s.GamePort), port(s.IgwPort), s.Restarts, orDash(s.ExitReason))
	}
	return tw.Flush()
}

// orNone renders an empty phase as "-" so a not-yet-populated .status reads clearly.
func orNone(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// port renders 0 (unset) as "-" rather than a misleading port number.
func port(p int) string {
	if p == 0 {
		return "-"
	}
	return strconv.Itoa(p)
}
