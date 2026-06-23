package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"go.muehmer.eu/dapdsm/pkg/domain/hostprep"
)

// stringList collects a repeatable string flag.
type stringList []string

func (s *stringList) String() string { return fmt.Sprintf("%v", []string(*s)) }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// prepareCmd runs the Phase-A cluster-access gate as a precondition, then
// applies the dune-user remediation (or prints it in --dry-run mode), and
// finally re-checks the host-prep state.
func prepareCmd(ctx context.Context, newRunner func(jump, kube string) hostprep.Runner, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("prepare-host", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jump := fs.String("jump", "", "jumphost ssh-config alias")
	kubeconfig := fs.String("kubeconfig", "", "kubeconfig path on the jumphost")
	uid := fs.Int("uid", 2000, "UID for the dune user")
	gid := fs.Int("gid", 2000, "GID for the dune group")
	dryRun := fs.Bool("dry-run", false, "print the remediation without applying it")
	var migrate stringList
	fs.Var(&migrate, "migrate-config", "config path on the jumphost to copy into dune's home (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *jump == "" || *kubeconfig == "" {
		fmt.Fprintln(stderr, "prepare-host: --jump and --kubeconfig are required")
		return ErrUsage
	}
	r := newRunner(*jump, *kubeconfig)

	// Precondition: the cluster-access gate must pass before we touch the host.
	gate := hostprep.ClusterGate(ctx, r)
	if !gate.Pass {
		for _, p := range gate.Probes {
			fmt.Fprintf(stdout, "  %s %s%s\n", mark(p.OK), p.Name, detail(p.Detail))
		}
		return fmt.Errorf("%w: not preparing host", ErrGateFailed)
	}

	opts := hostprep.Opts{User: "dune", UID: *uid, GID: *gid, Kubeconfig: *kubeconfig}
	steps := hostprep.PreparePlan(opts, migrate)
	if err := hostprep.Apply(ctx, r, steps, *dryRun, stdout); err != nil {
		return err
	}
	if !*dryRun {
		fmt.Fprintln(stdout, "== re-check ==")
		for _, c := range hostprep.HostChecks(ctx, r, opts) {
			fmt.Fprintf(stdout, "  %s %s%s\n", mark(c.OK), c.Name, detail(c.Detail))
		}
	}
	return nil
}
