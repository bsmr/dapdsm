package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"go.muehmer.eu/dapdsm/internal/pkg/dunectl/config"
	"go.muehmer.eu/dapdsm/pkg/domain/bootstrap"
	"go.muehmer.eu/dapdsm/pkg/transport/kube"
	"go.muehmer.eu/dapdsm/pkg/transport/kubedist"
	"go.muehmer.eu/dapdsm/pkg/transport/publicip"
	"go.muehmer.eu/dapdsm/pkg/transport/steamcmd"
)

// steamAdapter wraps the kubedist.Runner-based steamcmd functions to satisfy
// the bootstrap.SteamClient interface.
type steamAdapter struct{ r kubedist.Runner }

func (a steamAdapter) EnsureInstalled(ctx context.Context) error {
	return steamcmd.EnsureInstalled(ctx, a.r)
}

func (a steamAdapter) AppUpdate(ctx context.Context, appID uint32, dir string) error {
	return steamcmd.AppUpdate(ctx, a.r, appID, dir)
}

// isTTY reports whether w is a character device (interactive terminal).
// Returns false for any non-*os.File writer (e.g. bytes.Buffer in tests).
func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// hostCmd parses flags, loads the env file, wires real deps, and delegates to
// bootstrap.Run. It is the main entry point for the "host" subcommand.
func hostCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("ds-arrakis host", flag.ContinueOnError)
	fs.SetOutput(stderr)

	distro := fs.String("distro", "k3s", "Kubernetes distribution to install (k3s|rke2)")
	externalIP := fs.String("external-ip", "", "Node external IP (auto-detected if empty)")
	internalIP := fs.String("internal-ip", "", "Node internal IP (passed to K8s advertise address)")
	downloadDir := fs.String("download-dir", "/home/dune/.dune/download", "Steam depot download directory")
	envFile := fs.String("env-file", config.DefaultPath, "Path to dunectl.env config file")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %s", ErrUsage, err)
	}

	envCfg, err := config.LoadFromFile(*envFile)
	if err != nil {
		return fmt.Errorf("load env file %s: %w", *envFile, err)
	}

	cfg := bootstrap.Config{
		Target:      string(envCfg.Target),
		Distro:      *distro,
		ExternalIP:  *externalIP,
		InternalIP:  *internalIP,
		TLSSANs:     envCfg.K3SExtraTLSSANs,
		DownloadDir: *downloadDir,
	}

	runner := kubedist.NewRunner()
	dist, err := kubedist.New(cfg.Distro, runner, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "ds-arrakis host: invalid --distro: %s\n", err)
		return ErrUsage
	}

	deps := bootstrap.Deps{
		Distro:       dist,
		Resolver:     &publicip.HTTPResolver{},
		Steam:        steamAdapter{r: runner},
		Applier:      &kube.CmdRunner{Kubeconfig: dist.Kubeconfig(), Stderr: stderr},
		Stdout:       stdout,
		ReadyTimeout: 180 * time.Second,
	}

	bootstrap.PrintBanner(stdout, isTTY(stdout))
	return bootstrap.Run(ctx, cfg, deps)
}
