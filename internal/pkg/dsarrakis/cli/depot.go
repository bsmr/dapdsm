package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path"

	"go.muehmer.eu/dapdsm/pkg/domain/bootstrap"
	"go.muehmer.eu/dapdsm/pkg/domain/depot"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
	"go.muehmer.eu/dapdsm/pkg/transport/steamcmd"
)

// jumpRunner bridges the host-aware ssh.Client to the host-less steamcmd.Runner,
// binding every command to the jumphost.
type jumpRunner struct {
	ex   *ssh.Client
	host string
}

func (j jumpRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	res, err := j.ex.Run(ctx, j.host, name, args...)
	return res.Stdout, err
}

// realDepotRunner builds the production steamcmd.Runner: commands run on the
// jumphost via the system ssh client.
func realDepotRunner(jump string) steamcmd.Runner {
	return jumpRunner{ex: ssh.NewClient(), host: jump}
}

// depotCmd handles `ds-arrakis depot acquire --jump <alias> --env <prod|test> [--staging <dir>]`.
func depotCmd(ctx context.Context, newRunner func(jump string) steamcmd.Runner, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] != "acquire" {
		fmt.Fprintln(stderr, "usage: ds-arrakis depot acquire --jump <alias> --env <prod|test> [--staging <dir>]")
		return ErrUsage
	}
	fs := flag.NewFlagSet("depot acquire", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jump := fs.String("jump", "", "jumphost ssh-config alias")
	env := fs.String("env", "", "target environment: prod|test")
	staging := fs.String("staging", "", "depot staging dir on the jumphost (default /home/dune/depot/<env>)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *jump == "" || *env == "" {
		fmt.Fprintln(stderr, "depot: --jump and --env are required")
		return ErrUsage
	}
	appID, err := bootstrap.AppID(*env)
	if err != nil {
		return err
	}
	dir := *staging
	if dir == "" {
		dir = path.Join("/home/dune/depot", *env)
	}

	r := newRunner(*jump)
	fmt.Fprintf(stdout, "depot: acquiring %s (app %d) -> %s\n", *env, appID, dir)
	res, err := depot.Acquire(ctx, r, appID, dir)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "depot: version %s\n  operators:     %s\n  prerequisites: %s\n  battlegroup:   %s\n",
		res.Version, res.OperatorsDir, res.PrerequisitesDir, res.BattlegroupDir)
	return nil
}
