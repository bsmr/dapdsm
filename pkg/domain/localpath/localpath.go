// Package localpath installs Rancher's local-path-provisioner on a cluster that
// ships none (RKE2 does not bundle it; k3s does). The depot carries only the
// provisioner image (rancher/local-path-provisioner:v0.0.34), no manifest, so a
// pinned upstream manifest is embedded here. Parallel to operatorbringup's
// cert-manager knob: StorageClass is cluster infrastructure ds-arrakis owns.
package localpath

import (
	"context"
	"fmt"
)

// Kubectl is the cluster-access seam (Run + Apply), satisfied by ds-arrakis's
// kubectlAdapter over clusteraccess.Access.
type Kubectl interface {
	Run(ctx context.Context, args ...string) (string, error)
	Apply(ctx context.Context, manifest []byte) (string, error)
}

// Options configure the install.
type Options struct {
	DepotDir string // /home/dune/depot/<env> (provisioner tar under images/prerequisites/)
}

// Install loads the provisioner image (via the injected load func), applies the
// embedded manifest, and confirms a `local-path` StorageClass exists. Idempotent:
// apply is declarative; the StorageClass check is read-only.
func Install(ctx context.Context, kc Kubectl, load func(ctx context.Context) error, opts Options) error {
	if err := load(ctx); err != nil {
		return fmt.Errorf("load local-path-provisioner image: %w", err)
	}
	if _, err := kc.Apply(ctx, manifest); err != nil {
		return fmt.Errorf("apply local-path manifest: %w", err)
	}
	if _, err := kc.Run(ctx, "get", "storageclass", "local-path"); err != nil {
		return fmt.Errorf("local-path StorageClass not present after apply: %w", err)
	}
	return nil
}
