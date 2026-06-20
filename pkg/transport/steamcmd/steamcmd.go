// Package steamcmd drives the steamcmd binary to pull the Funcom Linux depot.
package steamcmd

import (
	"context"
	"fmt"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// EnsureInstalled installs steamcmd on the target via the embedded, distro-aware
// install-steamcmd.sh (Debian non-free/deb822 + Ubuntu multiverse, i386, debconf
// preseed, /usr/games symlink). It runs the script with `sudo bash -c` (bash for
// the script's herestrings/pipefail; sudo so it runs as root, skipping the
// script's own re-exec). Idempotent.
func EnsureInstalled(ctx context.Context, r Runner) error {
	if _, err := r.Run(ctx, "sudo", "bash", "-c", installScript); err != nil {
		return fmt.Errorf("ensure steamcmd: %w", err)
	}
	return nil
}

func AppUpdate(ctx context.Context, r Runner, appID uint32, installDir string) error {
	_, err := r.Run(ctx, "steamcmd",
		"+force_install_dir", installDir,
		"+login", "anonymous",
		"+app_update", fmt.Sprintf("%d", appID), "validate",
		"+quit")
	if err != nil {
		return fmt.Errorf("steamcmd app_update %d: %w", appID, err)
	}
	return nil
}
