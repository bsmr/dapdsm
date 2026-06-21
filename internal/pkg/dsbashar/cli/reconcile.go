package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"go.muehmer.eu/dapdsm/internal/pkg/dsbashar/config"
)

// reconcileDeps groups the per-step actions reconcile orchestrates. The
// default implementation forwards each step to the existing run*
// subcommands; tests inject recording fakes.
type reconcileDeps struct {
	cfg        config.Config
	initDB     func(ctx context.Context, stdout, stderr io.Writer) error
	patchBg    func(ctx context.Context, stdout, stderr io.Writer) error
	patchPorts func(ctx context.Context, gameBase, igwBase int, stdout, stderr io.Writer) error
	enableSet  func(ctx context.Context, m string, stdout, stderr io.Writer) error
	iniSet     func(ctx context.Context, key, value string, applyRestart bool, stdout, stderr io.Writer) error
	// bestEffortIniSet downgrades ini-set failures to warnings. The ini-set
	// path is single-node (local UserEngine.ini + vendor wrapper); multi-node
	// bring-up sets this so a missing-file failure does not abort an otherwise
	// complete bring-up. The K8s-native ini-set is deferred to block ②d.
	bestEffortIniSet bool
}

func reconcileCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	cfg, err := config.LoadFromFile(config.DefaultPath)
	if err != nil {
		return err
	}
	// Single-node: ini-set edits the local UserEngine.ini and must hard-fail.
	return reconcileWithConfig(ctx, cfg, args, false, stdout, stderr)
}

// reconcileWithConfig runs the reconcile pipeline against an explicit config
// instead of /etc/dune/dunectl.env. Multi-node bring-up uses this with the
// cluster-resident config (promoted to the dapdsm ConfigMap), since the
// workstation has no local dunectl.env, and with bestEffortIniSet=true so the
// single-node ini-set steps warn instead of aborting (K8s-native ini-set is ②d).
func reconcileWithConfig(ctx context.Context, cfg config.Config, args []string, bestEffortIniSet bool, stdout, stderr io.Writer) error {
	deps := reconcileDeps{
		cfg:              cfg,
		bestEffortIniSet: bestEffortIniSet,
		initDB:           func(ctx context.Context, out, errw io.Writer) error { return initDBCmd(ctx, nil, out, errw) },
		patchBg:          func(ctx context.Context, out, errw io.Writer) error { return patchBattlegroupCmd(ctx, nil, out, errw) },
		patchPorts: func(ctx context.Context, gb, ib int, out, errw io.Writer) error {
			return patchGamePortsCmd(ctx,
				[]string{"--game-base", fmt.Sprint(gb), "--igw-base", fmt.Sprint(ib)},
				out, errw)
		},
		enableSet: func(ctx context.Context, m string, out, errw io.Writer) error {
			return enableSetCmd(ctx, []string{m}, out, errw)
		},
		iniSet: func(ctx context.Context, key, value string, applyRestart bool, out, errw io.Writer) error {
			a := []string{key, value}
			if applyRestart {
				a = append([]string{"--apply", "--restart"}, a...)
			}
			return iniSetCmd(ctx, a, out, errw)
		},
	}
	return runReconcile(ctx, args, stdout, stderr, deps)
}

// runReconcile walks /etc/dune/dunectl.env and drives the post-bootstrap
// pipeline declaratively. Every step is opt-in via cfg keys (see
// dunectl.env.example); only the basics (init-db + patch-battlegroup)
// always run. Idempotent — each underlying subcommand is itself a no-op
// when the BG is already in the target state.
func runReconcile(ctx context.Context, args []string, stdout, stderr io.Writer, deps reconcileDeps) error {
	fs := flag.NewFlagSet("reconcile", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, "Usage: ds-bashar reconcile\n\n"+
			"Drives the full post-bootstrap pipeline (init-db, patch-battlegroup,\n"+
			"patch-game-ports, enable-set, ini-set ServerDisplayName / Password)\n"+
			"from /etc/dune/dunectl.env. Replaces the legacy post-bootstrap.sh.\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(reorderFlagArgs(fs, args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("reconcile: %w: %w", ErrUsage, err)
	}

	// 1. DB role / database (opt-out via SKIP_INIT_DB=true).
	if !deps.cfg.SkipInitDB {
		fmt.Fprintln(stdout, "reconcile: init-db")
		if err := deps.initDB(ctx, stdout, stderr); err != nil {
			return fmt.Errorf("reconcile init-db: %w", err)
		}
	}

	// 2. CR patches (HOST_DATACENTER_IP_ADDRESS + scheduler cleanup).
	fmt.Fprintln(stdout, "reconcile: patch-battlegroup")
	if err := deps.patchBg(ctx, stdout, stderr); err != nil {
		return fmt.Errorf("reconcile patch-battlegroup: %w", err)
	}

	// 3. Port shift — only when BOTH bases are present. An asymmetric
	// setting would only half-rewrite the per-set args and is almost
	// always a config typo, so we skip rather than risk a broken state.
	if deps.cfg.GamePortBase > 0 && deps.cfg.IGWPortBase > 0 {
		fmt.Fprintf(stdout, "reconcile: patch-game-ports (game=%d, igw=%d)\n",
			deps.cfg.GamePortBase, deps.cfg.IGWPortBase)
		if err := deps.patchPorts(ctx, deps.cfg.GamePortBase, deps.cfg.IGWPortBase, stdout, stderr); err != nil {
			return fmt.Errorf("reconcile patch-game-ports: %w", err)
		}
	}

	// 4. Always-on sets — sequentially, never parallel, to avoid the
	// S2S startup race documented in project_funcom-bootstrap-quirks.
	for _, m := range deps.cfg.AlwaysOnSets {
		fmt.Fprintf(stdout, "reconcile: enable-set %s\n", m)
		if err := deps.enableSet(ctx, m, stdout, stderr); err != nil {
			return fmt.Errorf("reconcile enable-set %s: %w", m, err)
		}
	}

	// 5. ServerDisplayName — apply without restart (cheap, no pods to
	// recycle for a display-name change).
	if deps.cfg.ServerDisplayName != "" {
		fmt.Fprintln(stdout, "reconcile: ini-set Bgd.ServerDisplayName")
		if err := deps.iniSet(ctx, "Bgd.ServerDisplayName", deps.cfg.ServerDisplayName, false, stdout, stderr); err != nil {
			if !deps.bestEffortIniSet {
				return fmt.Errorf("reconcile ini-set ServerDisplayName: %w", err)
			}
			fmt.Fprintf(stderr, "reconcile: WARN ServerDisplayName not applied (ini-set is single-node; deferred to ②d): %v\n", err)
		}
	}

	// 6. ServerLoginPassword — apply+restart (the password change has to
	// reach the live game-server pods).
	if deps.cfg.ServerPasswordFile != "" {
		pw, err := os.ReadFile(deps.cfg.ServerPasswordFile)
		if err != nil {
			return fmt.Errorf("reconcile read password file %s: %w",
				deps.cfg.ServerPasswordFile, err)
		}
		fmt.Fprintln(stdout, "reconcile: ini-set Bgd.ServerLoginPassword (--apply --restart)")
		if err := deps.iniSet(ctx, "Bgd.ServerLoginPassword", string(pw), true, stdout, stderr); err != nil {
			if !deps.bestEffortIniSet {
				return fmt.Errorf("reconcile ini-set ServerLoginPassword: %w", err)
			}
			fmt.Fprintf(stderr, "reconcile: WARN ServerLoginPassword not applied (ini-set is single-node; deferred to ②d): %v\n", err)
		}
	}

	fmt.Fprintln(stdout, "reconcile: done")
	return nil
}
