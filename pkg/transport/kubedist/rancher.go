package kubedist

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
)

// rancherDistro is the shared base for Rancher-family Kubernetes distributions
// (k3s, rke2). Fields capture the per-distro constants; methods encode the
// shared lifecycle logic.
type rancherDistro struct {
	name         string // e.g. "k3s"
	configDir    string // e.g. /etc/rancher/k3s
	dropinDir    string // e.g. /etc/rancher/k3s/config.yaml.d
	imagesDir    string // e.g. /var/lib/rancher/k3s/agent/images
	kubeconfig   string // e.g. /etc/rancher/k3s/k3s.yaml
	service      string // systemd service name, e.g. "k3s"
	installerURL string // e.g. https://get.k3s.io
	r            Runner
	stdout       io.Writer // reserved for progress output
}

// Name returns the distribution name.
func (d *rancherDistro) Name() string { return d.name }

// Kubeconfig returns the path to the kubeconfig written by the distribution.
func (d *rancherDistro) Kubeconfig() string { return d.kubeconfig }

// Install writes the base config and drop-in files, runs the upstream installer
// script, and enables the systemd service.
func (d *rancherDistro) Install(ctx context.Context, cfg Config) error {
	base, err := baseConfig(d.name)
	if err != nil {
		return err
	}
	if err := d.writeFile(ctx, path.Join(d.configDir, "config.yaml"), base); err != nil {
		return err
	}
	dropins, err := renderDropins(d.name, cfg)
	if err != nil {
		return err
	}
	for name, body := range dropins {
		if err := d.writeFile(ctx, path.Join(d.dropinDir, name), body); err != nil {
			return err
		}
	}
	// Remove a stale 30-internal-routing.yaml when DNAT routing is not needed.
	if _, hasRouting := dropins["30-internal-routing.yaml"]; !hasRouting {
		_, _ = d.r.Run(ctx, "rm", "-f", path.Join(d.dropinDir, "30-internal-routing.yaml"))
	}
	if _, err := d.r.Run(ctx, "sh", "-c", fmt.Sprintf("curl -sfL %s | sh -", d.installerURL)); err != nil {
		return fmt.Errorf("%s installer: %w", d.name, err)
	}
	if _, err := d.r.Run(ctx, "systemctl", "enable", "--now", d.service); err != nil {
		return fmt.Errorf("enable %s: %w", d.service, err)
	}
	return nil
}

// writeFile writes body to dest on the target node via the runner,
// creating parent directories as needed.
func (d *rancherDistro) writeFile(ctx context.Context, dest string, body []byte) error {
	if _, err := d.r.Run(ctx, "install", "-d", "-m", "0755", path.Dir(dest)); err != nil {
		return err
	}
	// Here-doc sentinel DSEOF must not appear on a line by itself in body; our embedded YAML never does.
	_, err := d.r.Run(ctx, "sh", "-c",
		fmt.Sprintf("cat > '%s' <<'DSEOF'\n%s\nDSEOF\nchmod 0644 '%s'", dest, strings.TrimRight(string(body), "\n"), dest))
	return err
}

// EnsureReady polls kubectl until the API server responds or ctx is cancelled.
func (d *rancherDistro) EnsureReady(ctx context.Context) error {
	for {
		if _, err := d.r.Run(ctx, "kubectl", "--kubeconfig", d.kubeconfig, "get", "nodes"); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s API not ready: %w", d.name, ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}
}

// ImportImages copies tar archives from tarDir into the distribution's
// pre-pull images directory and waits for the node to become ready.
func (d *rancherDistro) ImportImages(ctx context.Context, tarDir string) error {
	if _, err := d.r.Run(ctx, "install", "-d", "-m", "0755", d.imagesDir); err != nil {
		return err
	}
	if _, err := d.r.Run(ctx, "sh", "-c",
		fmt.Sprintf("install -m 0644 '%s'/*.tar '%s'/", tarDir, d.imagesDir)); err != nil {
		return fmt.Errorf("import images: %w", err)
	}
	return d.EnsureReady(ctx)
}

// Uninstall runs the distribution's uninstall script if present.
func (d *rancherDistro) Uninstall(ctx context.Context) error {
	script := fmt.Sprintf("/usr/local/bin/%s-uninstall.sh", d.name)
	_, err := d.r.Run(ctx, "sh", "-c",
		fmt.Sprintf("[ -x %s ] && %s || true", script, script))
	return err
}
