// Package steamcmd drives the steamcmd binary to pull the Funcom Linux depot.
package steamcmd

import (
	"context"
	"fmt"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

func EnsureInstalled(ctx context.Context, r Runner) error {
	_, err := r.Run(ctx, "sh", "-c",
		"command -v steamcmd || (sudo add-apt-repository -y multiverse && "+
			"sudo dpkg --add-architecture i386 && sudo apt-get update && "+
			"sudo apt-get install -y steamcmd)")
	if err != nil {
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
