package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"go.muehmer.eu/dapdsm/pkg/transport/database"
	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

type initDBDeps struct {
	runner kube.Runner
}

func defaultInitDBDeps(stderr io.Writer) initDBDeps {
	return initDBDeps{runner: newKubeRunner(stderr)}
}

func initDBCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return runInitDB(ctx, args, stdout, stderr, defaultInitDBDeps(stderr))
}

// runInitDB provisions the per-app Postgres role and database that the
// Funcom database-operator's util-pod requires before it can initialise
// the schema. The legacy dune-setup.sh did this from
// action_patch_battlegroup; the ds-bashar migration dropped it, so a fresh
// BG hangs at "Schema not found, initializing" until this is run.
//
// Idempotent — re-run on an already-initialised BG simply re-aligns the
// super-user password.
func runInitDB(ctx context.Context, args []string, stdout, stderr io.Writer, deps initDBDeps) error {
	fs := flag.NewFlagSet("init-db", flag.ContinueOnError)
	fs.SetOutput(stderr)
	ns := fs.String("namespace", "", "BattleGroup namespace (default: first funcom-seabass-*)")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: ds-bashar init-db [flags]\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("init-db: %w: %w", ErrUsage, err)
	}

	if *ns == "" {
		found, err := kube.FindBattleGroupNamespace(ctx, deps.runner)
		if err != nil {
			return err
		}
		*ns = found
	}

	creds, err := database.ResolveCreds(ctx, deps.runner, *ns)
	if err != nil {
		return err
	}
	if err := database.InitGameUser(ctx, deps.runner, *ns, creds); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "init-db: role %q + database %q provisioned on pod %s\n",
		creds.GameUser, creds.Database, creds.Pod)
	return nil
}
