package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

type preShutdownDeps struct {
	runner       kube.Runner
	runVendor    vendorRunner
	announceDeps announceDeps
	pollEvery    time.Duration // default 5s
	timeout      time.Duration // default 3min
}

func preShutdownCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return runPreShutdown(ctx, args, stdout, stderr, preShutdownDeps{
		runner:       &kube.CmdRunner{Stderr: stderr},
		runVendor:    execVendor,
		announceDeps: defaultAnnounceDeps(),
	})
}

// runPreShutdown drains the BattleGroup before the host shuts down or
// restarts: it calls Funcom's `battlegroup stop` and then waits for the
// CR phase to reach Stopped and the game-server pods to drain. Designed
// for `ExecStop=` on k3s.service — that's why a timeout returns nil (a
// non-zero exit there would convince systemd to escalate to SIGKILL,
// which is exactly the world_partition-corruption scenario we are
// trying to prevent).
func runPreShutdown(ctx context.Context, args []string, stdout, stderr io.Writer, deps preShutdownDeps) error {
	fs := flag.NewFlagSet("pre-shutdown", flag.ContinueOnError)
	fs.SetOutput(stderr)
	bin := fs.String("bg-binary", DefaultBattlegroupBin, "Path to Funcom's battlegroup wrapper")
	ns := fs.String("namespace", "", "BattleGroup namespace (default: first funcom-seabass-*)")
	bg := fs.String("bg-name", "", "BattleGroup name (default: derived from --namespace)")
	timeoutFlag := fs.Duration("timeout", 0, "Overall wait budget (default 3m; set 0 to use the dep default)")
	announce := fs.Duration("announce", 0, "Announce a shutdown countdown of this duration, then act (e.g. 5m)")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: ds-bashar pre-shutdown\n\n"+
			"Calls `battlegroup stop` and waits for the BG to fully drain.\n"+
			"Designed for ExecStop= on k3s.service: timeouts log a warning\n"+
			"but exit zero so systemd does not escalate to SIGKILL.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(reorderFlagArgs(fs, args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("pre-shutdown: %w: %w", ErrUsage, err)
	}

	if deps.pollEvery == 0 {
		deps.pollEvery = 5 * time.Second
	}
	if deps.timeout == 0 {
		deps.timeout = 3 * time.Minute
	}
	if *timeoutFlag > 0 {
		deps.timeout = *timeoutFlag
	}

	if *ns == "" {
		if found, err := kube.FindBattleGroupNamespace(ctx, deps.runner); err == nil {
			*ns = found
		}
	}
	if *bg == "" && *ns != "" {
		*bg = kube.BattleGroupName(*ns)
	}

	return withAnnounce(ctx, *announce, "Maintenance", func(ctx context.Context) error {
		// Step 1: ask Funcom's wrapper to stop the BG.
		fmt.Fprintln(stdout, "pre-shutdown: battlegroup stop")
		if err := deps.runVendor(ctx, *bin, "stop", stdout, stderr); err != nil {
			return err
		}

		// Step 2: poll until status.phase = Stopped.
		deadline := time.Now().Add(deps.timeout)
		for time.Now().Before(deadline) {
			phase, _ := deps.runner.Get(ctx, "battlegroup", *bg, "-n", *ns,
				"-o", "jsonpath={.status.phase}")
			if strings.EqualFold(strings.TrimSpace(string(phase)), "Stopped") {
				fmt.Fprintln(stdout, "pre-shutdown: phase = Stopped")
				break
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(deps.pollEvery):
			}
		}
		if !time.Now().Before(deadline) {
			fmt.Fprintln(stderr, "pre-shutdown: timeout waiting for phase=Stopped — proceeding anyway")
		}

		// Step 3: poll until no game-server pods remain.
		deadline = time.Now().Add(deps.timeout / 2)
		for time.Now().Before(deadline) {
			out, _ := deps.runner.Get(ctx, "pods", "-n", *ns,
				"-l", "role=igw-server", "-o", "name")
			if strings.TrimSpace(string(out)) == "" {
				fmt.Fprintln(stdout, "pre-shutdown: game-server pods drained")
				break
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(deps.pollEvery):
			}
		}
		if !time.Now().Before(deadline) {
			fmt.Fprintln(stderr, "pre-shutdown: timeout waiting for game-server pods to drain — proceeding anyway")
		}

		fmt.Fprintln(stdout, "pre-shutdown: done")
		return nil
	}, deps.announceDeps)
}
