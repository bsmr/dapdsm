// Package bootstrap orchestrates a fresh VM into a Dune-ready Kubernetes
// host: distro install -> ready -> Steam depot -> operator images -> CRDs.
// It then hands off to dunectl (Funcom server lifecycle).
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.muehmer.eu/dapdsm/pkg/transport/kubedist"
	"go.muehmer.eu/dapdsm/pkg/transport/publicip"
)

var ErrNoOperators = errors.New("operator payload missing under <download>/images/operators")

func AppID(target string) (uint32, error) {
	switch target {
	case "prod":
		return 4754530, nil
	case "test":
		return 3104830, nil
	default:
		return 0, fmt.Errorf("unknown target %q (want prod|test)", target)
	}
}

type Config struct {
	Target      string
	Distro      string
	ExternalIP  string
	InternalIP  string
	TLSSANs     []string
	DownloadDir string
}

type SteamClient interface {
	EnsureInstalled(ctx context.Context) error
	AppUpdate(ctx context.Context, appID uint32, installDir string) error
}

type CRDApplier interface {
	Apply(ctx context.Context, args ...string) error
}

type Deps struct {
	Distro       kubedist.Distro
	Resolver     publicip.Resolver
	Steam        SteamClient
	Applier      CRDApplier
	Stdout       io.Writer
	ReadyTimeout time.Duration
}

func Run(ctx context.Context, cfg Config, d Deps) error {
	appID, err := AppID(cfg.Target)
	if err != nil {
		return err
	}
	ext := cfg.ExternalIP
	if ext == "" {
		if ext, err = d.Resolver.Resolve(ctx); err != nil {
			return fmt.Errorf("resolve external IP: %w", err)
		}
	}
	kcfg := kubedist.Config{ExternalIP: ext, InternalIP: cfg.InternalIP, TLSSANs: cfg.TLSSANs}

	if err := d.Distro.Install(ctx, kcfg); err != nil {
		return fmt.Errorf("distro install: %w", err)
	}
	readyCtx, cancel := context.WithTimeout(ctx, d.ReadyTimeout)
	defer cancel()
	if err := d.Distro.EnsureReady(readyCtx); err != nil {
		return fmt.Errorf("distro ready: %w", err)
	}
	if err := d.Steam.EnsureInstalled(ctx); err != nil {
		return fmt.Errorf("steam install: %w", err)
	}
	if err := d.Steam.AppUpdate(ctx, appID, cfg.DownloadDir); err != nil {
		return fmt.Errorf("steam app-update: %w", err)
	}

	opDir := filepath.Join(cfg.DownloadDir, "images", "operators")
	if _, err := os.Stat(filepath.Join(opDir, "crds")); err != nil {
		return fmt.Errorf("%w: %s", ErrNoOperators, opDir)
	}
	if err := d.Distro.ImportImages(ctx, opDir); err != nil {
		return fmt.Errorf("import images: %w", err)
	}
	if err := d.Applier.Apply(ctx, "-f", filepath.Join(opDir, "crds")); err != nil {
		return fmt.Errorf("apply CRDs/RBAC: %w", err)
	}

	fmt.Fprintln(d.Stdout, "ds-arrakis: Dune-ready. Next: sudo dunectl setup && sudo dunectl reconcile")
	return nil
}
