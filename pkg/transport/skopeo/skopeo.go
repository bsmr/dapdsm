// Package skopeo wraps the jumphost-side `skopeo` CLI: install it, read a
// docker-archive's image tags, and copy archives into a (possibly insecure)
// registry. It is host-less — all exec goes through an injected Runner, which a
// jumphost-bound adapter supplies. See the S2 image-distribution spec.
package skopeo

import (
	"context"
	"encoding/json"
	"fmt"
)

// Runner runs a command on the target host and returns its stdout.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// installScript installs skopeo idempotently. skopeo is in Debian-13 `main` and
// Ubuntu `main`/`universe`, so a plain apt install suffices — no i386, non-free,
// or debconf preseed (unlike steamcmd).
const installScript = `set -euo pipefail
command -v skopeo >/dev/null 2>&1 && exit 0
apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y skopeo`

// EnsureInstalled installs skopeo on the target if missing. It runs via
// `sudo bash -c` (bash for set -o pipefail; sudo to install as root, a no-op for
// root). Idempotent.
func EnsureInstalled(ctx context.Context, r Runner) error {
	if _, err := r.Run(ctx, "sudo", "bash", "-c", installScript); err != nil {
		return fmt.Errorf("ensure skopeo: %w", err)
	}
	return nil
}

// dockerArchiveManifest is the docker-archive manifest.json element shape.
type dockerArchiveManifest struct {
	RepoTags []string `json:"RepoTags"`
}

// RepoTags returns the repo:tag references inside a docker-archive tar by reading
// its manifest.json. A single archive may bundle several images.
func RepoTags(ctx context.Context, r Runner, tarPath string) ([]string, error) {
	out, err := r.Run(ctx, "tar", "-xOf", tarPath, "manifest.json")
	if err != nil {
		return nil, fmt.Errorf("read manifest of %s: %w", tarPath, err)
	}
	var manifests []dockerArchiveManifest
	if err := json.Unmarshal([]byte(out), &manifests); err != nil {
		return nil, fmt.Errorf("parse manifest of %s: %w", tarPath, err)
	}
	var tags []string
	for _, m := range manifests {
		tags = append(tags, m.RepoTags...)
	}
	if len(tags) == 0 {
		return nil, fmt.Errorf("no RepoTags in %s", tarPath)
	}
	return tags, nil
}

// Copy runs `skopeo copy --dest-tls-verify=false <src> <dst>`. The flag is set
// because the in-cluster registry is insecure HTTP (trust is delegated to the
// provisioner-seeded registries.yaml on the nodes).
func Copy(ctx context.Context, r Runner, src, dst string) error {
	if _, err := r.Run(ctx, "skopeo", "copy", "--dest-tls-verify=false", src, dst); err != nil {
		return fmt.Errorf("skopeo copy %s -> %s: %w", src, dst, err)
	}
	return nil
}
