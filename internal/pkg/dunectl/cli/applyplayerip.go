package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"

	"go.muehmer.eu/dapdsm/internal/pkg/kube"
	"go.muehmer.eu/dapdsm/internal/pkg/publicip"
)

// DefaultSettingsConf is the path Funcom's vendor scripts read at server-pod
// startup. Line 4 is the IP used for -ExternalAddress.
const DefaultSettingsConf = "/home/dune/.dune/settings.conf"

// IGWServerSelector matches the game-server pods the Funcom operators
// label with role=igw-server. Recreating these pods is what makes them
// pick up the new -ExternalAddress after settings.conf has been updated.
const IGWServerSelector = "role=igw-server"

// applyPlayerIPDeps groups external dependencies of applyPlayerIP for testing.
type applyPlayerIPDeps struct {
	runner     kube.Runner
	resolver   publicip.Resolver
	restartK3s func(ctx context.Context) error
	writeFile  func(path string, content []byte) error
}

func defaultApplyPlayerIPDeps(stderr io.Writer) applyPlayerIPDeps {
	return applyPlayerIPDeps{
		runner:     &kube.CmdRunner{Stderr: stderr},
		resolver:   &publicip.HTTPResolver{},
		restartK3s: restartK3sViaSudo,
		writeFile:  writeFile0644,
	}
}

func restartK3sViaSudo(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "sudo", "systemctl", "restart", "k3s")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo systemctl restart k3s: %w", err)
	}
	return nil
}

func writeFile0644(path string, content []byte) error {
	return os.WriteFile(path, content, 0o644)
}

func applyPlayerIPCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return applyPlayerIP(ctx, args, stdout, stderr, defaultApplyPlayerIPDeps(stderr))
}

func applyPlayerIP(ctx context.Context, args []string, stdout, stderr io.Writer, deps applyPlayerIPDeps) error {
	fs := flag.NewFlagSet("apply-player-ip", flag.ContinueOnError)
	fs.SetOutput(stderr)
	ip := fs.String("ip", "", "Player-facing IPv4 (default: auto-detect via api.ipify.org)")
	ns := fs.String("namespace", "", "BattleGroup namespace for pod recreate (default: first funcom-seabass-*)")
	conf := fs.String("settings-conf", DefaultSettingsConf, "Path to Funcom's settings.conf")
	skipRestart := fs.Bool("skip-restart", false, "Skip 'sudo systemctl restart k3s'")
	skipPodRecreate := fs.Bool("skip-pod-recreate", false, "Skip deleting role=igw-server pods")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("apply-player-ip: %w: %w", ErrUsage, err)
	}

	if *ip == "" {
		resolved, err := deps.resolver.Resolve(ctx)
		if err != nil {
			return fmt.Errorf("resolve public IPv4: %w", err)
		}
		*ip = resolved
	}
	if *ns == "" && !*skipPodRecreate {
		found, err := kube.FindBattleGroupNamespace(ctx, deps.runner)
		if err != nil {
			return err
		}
		*ns = found
	}

	fmt.Fprintf(stdout, "player IP:     %s\n", *ip)
	fmt.Fprintf(stdout, "settings.conf: %s\n", *conf)
	if *ns != "" {
		fmt.Fprintf(stdout, "namespace:     %s\n", *ns)
	}

	body := SettingsConfBody(*ip)
	if err := deps.writeFile(*conf, body); err != nil {
		return fmt.Errorf("write %s: %w", *conf, err)
	}
	fmt.Fprintf(stdout, "wrote settings.conf (%d bytes)\n", len(body))

	switch {
	case *skipRestart:
		fmt.Fprintln(stdout, "skipping k3s restart (--skip-restart)")
	default:
		fmt.Fprintln(stdout, "restarting k3s")
		if err := deps.restartK3s(ctx); err != nil {
			return err
		}
	}

	switch {
	case *skipPodRecreate:
		fmt.Fprintln(stdout, "skipping pod recreate (--skip-pod-recreate)")
	default:
		fmt.Fprintf(stdout, "deleting pods labelled %s in %s\n", IGWServerSelector, *ns)
		if err := deps.runner.DeletePods(ctx, *ns, IGWServerSelector); err != nil {
			return err
		}
	}

	fmt.Fprintln(stdout, "done")
	return nil
}

// SettingsConfBody returns the Funcom-mandated settings.conf payload:
// three blank lines, then the IP, then a newline. Line 4 (the IP) is the
// value /usr/local/bin/k3s-custom-runner.sh reads at server-pod startup
// to set -ExternalAddress on the game-server process.
func SettingsConfBody(ip string) []byte {
	return fmt.Appendf(nil, "\n\n\n%s\n", ip)
}
