// Package depot acquires the Funcom dedicated-server depot onto the control host
// via anonymous SteamCMD, producing the operator/prerequisite/battlegroup image
// tars + version. It performs no node distribution (a later slice). See the spec.
package depot

import (
	"context"
	"fmt"
	"path"
	"strings"

	"go.muehmer.eu/dapdsm/pkg/transport/steamcmd"
)

// Result is the outcome of a depot acquisition: the resolved Funcom version and
// the image directories under the staging dir.
type Result struct {
	Version          string
	OperatorsDir     string
	PrerequisitesDir string
	BattlegroupDir   string
}

// Acquire ensures steamcmd is installed, downloads the depot for appID into
// stagingDir (anonymous, validated), reads the operator version.txt, and returns
// the resolved version + image dir paths. It never opens files itself; all I/O is
// via the runner.
func Acquire(ctx context.Context, r steamcmd.Runner, appID uint32, stagingDir string) (Result, error) {
	if err := steamcmd.EnsureInstalled(ctx, r); err != nil {
		return Result{}, fmt.Errorf("depot: %w", err)
	}
	if err := steamcmd.AppUpdate(ctx, r, appID, stagingDir); err != nil {
		return Result{}, fmt.Errorf("depot: %w", err)
	}
	images := path.Join(stagingDir, "images")
	verPath := path.Join(images, "operators", "version.txt")
	out, err := r.Run(ctx, "cat", verPath)
	if err != nil {
		return Result{}, fmt.Errorf("depot: read %s: %w", verPath, err)
	}
	return Result{
		Version:          strings.TrimSpace(out),
		OperatorsDir:     path.Join(images, "operators"),
		PrerequisitesDir: path.Join(images, "prerequisites"),
		BattlegroupDir:   path.Join(images, "battlegroup"),
	}, nil
}
