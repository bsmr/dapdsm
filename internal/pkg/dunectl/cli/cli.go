// Package cli implements the dunectl command-line interface.
//
// Run dispatches an argv tail to the matching subcommand. Subcommands receive
// a context plus injected stdin/stdout/stderr and never call os.Exit; they
// return errors that the caller converts into a process exit status.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// ErrUsage is returned when the caller invoked dunectl with no subcommand or
// with an unknown one. main maps it to exit code 2.
var ErrUsage = errors.New("usage error")

// Run dispatches args[0] to the matching subcommand.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return ErrUsage
	}

	switch args[0] {
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	case "patch-battlegroup":
		return patchBattlegroupCmd(ctx, args[1:], stdout, stderr)
	case "init-db":
		return initDBCmd(ctx, args[1:], stdout, stderr)
	case "setup":
		return setupCmd(ctx, args[1:], stdout, stderr)
	case "patch-game-ports":
		return patchGamePortsCmd(ctx, args[1:], stdout, stderr)
	case "pre-shutdown":
		return preShutdownCmd(ctx, args[1:], stdout, stderr)
	case "heal-stuck-pods":
		return healStuckPodsCmd(ctx, args[1:], stdout, stderr)
	case "reconcile":
		return reconcileCmd(ctx, args[1:], stdout, stderr)
	case "apply-player-ip":
		return applyPlayerIPCmd(ctx, args[1:], stdout, stderr)
	case "env":
		return envCmd(ctx, args[1:], stdout, stderr)
	case "list-sets":
		return listSetsCmd(ctx, args[1:], stdout, stderr)
	case "enable-set":
		return enableSetCmd(ctx, args[1:], stdout, stderr)
	case "disable-set":
		return disableSetCmd(ctx, args[1:], stdout, stderr)
	case "start":
		return startCmd(ctx, args[1:], stdout, stderr)
	case "stop":
		return stopCmd(ctx, args[1:], stdout, stderr)
	case "restart":
		return restartCmd(ctx, args[1:], stdout, stderr)
	case "ini-set":
		return iniSetCmd(ctx, args[1:], stdout, stderr)
	case "apply-user-settings":
		return applyUserSettingsCmd(ctx, args[1:], stdout, stderr)
	case "update":
		return updateCmd(ctx, args[1:], stdout, stderr)
	case "version", "-v", "--version":
		return versionCmd(ctx, args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown subcommand %q (try \"dunectl help\"): %w", args[0], ErrUsage)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, usage)
}

const usage = `dunectl — orchestrate Dune Awakening private dedicated servers.

Usage:
  dunectl <command> [arguments]

Commands:
  help                Print this message.
  version             Print build identity (semver, commit, Go runtime).
  patch-battlegroup   Patch HOST_DATACENTER_IP_ADDRESS and strip the
                      memory-focused-scheduler from the live BattleGroup CR.
                      Use --help on the subcommand for flags.
  init-db             Provision the per-app Postgres role and database
                      the Funcom database-operator's util-pod expects to
                      exist. Mandatory on every fresh BattleGroup.
                      Idempotent.
  setup               Drive Funcom's setup.sh non-interactively. Reads
                      WORLD_NAME and WORLD_REGION from dunectl.env plus
                      the FLS token, validates them, and pipes the input
                      in the order setup/world.sh expects.
  patch-game-ports    Shift the per-set UDP base ports off Funcom's
                      hardcoded 7777/7888. Required when two BGs share
                      one public IP behind a vCD-style edge so they
                      don't both try to bind the same port on different
                      VMs. Flags: --game-base N --igw-base M (required).
  pre-shutdown        Drain the BattleGroup before host shutdown.
                      Designed for ExecStop= on k3s.service; timeouts
                      log a warning but exit zero so systemd doesn't
                      escalate to SIGKILL.
  heal-stuck-pods     Flag (or, with --apply, force-delete) game-server
                      pods stuck Not-Ready past --threshold-minutes
                      (default 10) and util-pods in Phase=Failed/Error.
                      Dry-run by default.
  reconcile           Drive the full post-bootstrap pipeline declaratively
                      from /etc/dune/dunectl.env (init-db, patch-battlegroup,
                      patch-game-ports, enable-set, ini-set). Idempotent.
                      Replaces the legacy post-bootstrap.sh.
  apply-player-ip     Write Funcom's settings.conf, restart k3s, and
                      recreate role=igw-server pods so -ExternalAddress
                      picks up the new value. Use --help for flags.
  env                 Print the parsed /etc/dune/dunectl.env (target,
                      FLS token path, derived Steam app id, extra SANs).
  list-sets           Show every map in the BattleGroup with scaling
                      mode, replicas, and partition assignment.
                      --json for machine-readable output.
  enable-set <map>... Make the given map(s) always-on
                      (dedicatedScaling=false, replicas=1; --replicas N
                      overrides).
  disable-set <map>.. Return the given map(s) to on-demand
                      (dedicatedScaling=true, replicas=0).
  start | stop |      Drive the BattleGroup lifecycle through Funcom's
   restart            vendor wrapper at /home/dune/.dune/bin/battlegroup
                      (orchestrates operator timing correctly).
  ini-set <key> <val> Set a key in a Funcom INI file (default:
                      UserEngine.ini, section [ConsoleVariables]).
                      String values are auto-quoted; --raw to pass
                      through. --apply runs apply-default-usersettings;
                      --restart implies --apply plus a restart.
  apply-user-settings Re-deploy UserEngine.ini / UserGame.ini to the
                      filebrowser pod via 'battlegroup
                      apply-default-usersettings'. --restart adds a
                      'battlegroup restart' afterwards. Use this when
                      the INIs were edited out-of-band and the BG-CR
                      lost its DisplayName / Password defaults.
  update              Run a Funcom update end-to-end: 'update'
                      (downloads from Steam + applies) → 'apply-
                      default-usersettings' → 'restart'.
                      --from-downloads switches the first step to
                      'update-from-downloads' (apply the already-
                      downloaded version). --no-restart stops after
                      the apply. This is the post-update hook that
                      prevents Sietches from losing their DisplayName
                      after a 'battlegroup update'.

Operator configuration is read from /etc/dune/dunectl.env;
see etc/dune/dunectl.env.example for the supported keys.
`
