package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"go.muehmer.eu/dapdsm/pkg/config"
	"go.muehmer.eu/dapdsm/pkg/domain/rollout"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

func rolloutCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "rollout: expected exactly one <host>")
		return ErrUsage
	}
	host := args[0]
	dir, err := config.ConfigDir(os.Getenv)
	if err != nil {
		return err
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	deps := rollout.Deps{
		SSH:        ssh.NewClient(),
		Build:      realBuild,
		EtcArchive: realEtcArchive,
		Stdout:     stdout,
	}
	return rollout.Run(ctx, deps, cfg, dir, host)
}

// realBuild cross-builds a suite binary for linux/amd64 into bin/.
func realBuild(ctx context.Context, binary string) (string, error) {
	out := "bin/" + binary + "-linux-amd64"
	cmd := exec.CommandContext(ctx, "go", "build", "-o", out, "./cmd/"+binary)
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build %s: %w (%s)", binary, err, b)
	}
	return out, nil
}

// realEtcArchive tars the repo's etc/ dir to a byte buffer.
func realEtcArchive(ctx context.Context) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "tar", "-c", "etc")
	return cmd.Output()
}
