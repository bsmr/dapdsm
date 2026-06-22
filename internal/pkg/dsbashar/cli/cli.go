// Package cli implements the ds-bashar command-line interface.
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
	"strings"

	"go.muehmer.eu/dapdsm/pkg/transport/clusteraccess"
)

// ErrUsage is returned when the caller invoked ds-bashar with no subcommand or
// with an unknown one. main maps it to exit code 2.
var ErrUsage = errors.New("usage error")

// Run dispatches args[0] to the matching subcommand. Leading global flags
// --jump and --kubeconfig are stripped before the verb is dispatched. ex is the
// SSH execer used to build the jumphost kube runner when --jump is set; pass
// ssh.NewClient() in production, a stub in tests.
func Run(ctx context.Context, args []string, ex clusteraccess.Execer, stdin io.Reader, stdout, stderr io.Writer) error {
	jump, kubeconfig, rest := parseGlobals(args)
	configureRunner(ex, jump, kubeconfig)
	args = rest
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
	case "status":
		return statusCmd(ctx, args[1:], stdout, stderr)
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
	case "broadcast":
		return broadcastCmd(ctx, args[1:], stdout, stderr)
	case "discover":
		return discoverCmd(ctx, args[1:], stdout, stderr)
	case "bringup":
		return bringupCmd(ctx, args[1:], stdin, stdout, stderr)
	case "version", "-v", "--version":
		return versionCmd(ctx, args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown subcommand %q (try \"ds-bashar help\"): %w", args[0], ErrUsage)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, usage)
}

// parseGlobals consumes leading --jump/--kubeconfig (space or =) and returns
// the remaining argv. Unknown leading tokens stop parsing (the verb begins).
func parseGlobals(args []string) (jump, kubeconfig string, rest []string) {
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--jump" && i+1 < len(args):
			jump, i = args[i+1], i+2
		case strings.HasPrefix(a, "--jump="):
			jump, i = strings.TrimPrefix(a, "--jump="), i+1
		case a == "--kubeconfig" && i+1 < len(args):
			kubeconfig, i = args[i+1], i+2
		case strings.HasPrefix(a, "--kubeconfig="):
			kubeconfig, i = strings.TrimPrefix(a, "--kubeconfig="), i+1
		default:
			return jump, kubeconfig, args[i:]
		}
	}
	return jump, kubeconfig, args[i:]
}

const usage = `ds-bashar — orchestrate Dune Awakening private dedicated servers.

Usage:
  ds-bashar <command> [arguments]

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
  status              Show the BattleGroup's observed status: overall /
                      serverGroup / db / director phases and the per-map
                      server table. --json for machine-readable output.
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
  broadcast [--ssh A]  Send an in-game banner. Default: local kubectl on the
   <text>              node; --ssh <alias> publishes over SSH. --title, --duration.
  discover            List BattleGroups discovered on the cluster.
  bringup             Orchestrate the full multi-node bring-up: resolve
                      config (flags or wizard) → promote it to the cluster
                      → gate on BattleGroup discovery → run Funcom's
                      setup.sh on the jumphost (fresh clusters only) →
                      load the BG runtime image → init-db → reconcile.
                      Multi-node only: requires the global --jump flag.
                      Flags: --name --display --region --fls-token
                      --no-input.

Global flags --jump <alias> and --kubeconfig <path> precede the verb
(e.g. ds-bashar --jump jh bringup --name Arrakis …). They select the
jumphost the cluster is reached through; without --jump verbs use the
local on-node kubectl.

Operator configuration is read from /etc/dune/dunectl.env;
see etc/dune/dunectl.env.example for the supported keys.
`
