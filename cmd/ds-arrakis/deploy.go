package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// binaryBuilder compiles the ds-arrakis binary for a target platform.
type binaryBuilder interface {
	Build(ctx context.Context, outPath string) error
}

// deploySSH is the subset of ssh.Client methods used by deploy.
type deploySSH interface {
	SendFile(ctx context.Context, host, local, remote string) error
	Run(ctx context.Context, host, cmd string, args ...string) (ssh.Result, error)
}

// goBuilder is the real binaryBuilder: cross-compiles for linux/amd64.
type goBuilder struct{}

func (goBuilder) Build(ctx context.Context, outPath string) error {
	cmd := exec.CommandContext(ctx, "go", "build", "-o", outPath, "./cmd/ds-arrakis")
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build: %w: %s", err, out)
	}
	return nil
}

// deploy builds the linux/amd64 binary, ships it to the remote host, installs
// it into /usr/local/bin/ds-arrakis, then invokes it as
// "sudo ds-arrakis host <hostArgs...>". Progress lines are written to stdout.
func deploy(
	ctx context.Context,
	sshClient deploySSH,
	builder binaryBuilder,
	host string,
	hostArgs []string,
	stdout io.Writer,
) error {
	// Build to a temporary path on the local machine.
	tmpDir, err := os.MkdirTemp("", "ds-arrakis-deploy-*")
	if err != nil {
		return fmt.Errorf("deploy: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	localBin := tmpDir + "/ds-arrakis"
	fmt.Fprintf(stdout, "deploy: building linux/amd64 binary → %s\n", localBin)
	if err := builder.Build(ctx, localBin); err != nil {
		return fmt.Errorf("deploy: build: %w", err)
	}

	// Upload to a remote temp path, then install with correct permissions.
	remoteTmp := "/tmp/ds-arrakis.deploy"
	fmt.Fprintf(stdout, "deploy: sending binary to %s:%s\n", host, remoteTmp)
	if err := sshClient.SendFile(ctx, host, localBin, remoteTmp); err != nil {
		return fmt.Errorf("deploy: send file: %w", err)
	}

	const installDst = "/usr/local/bin/ds-arrakis"
	fmt.Fprintf(stdout, "deploy: installing to %s on %s\n", installDst, host)
	if _, err := sshClient.Run(ctx, host, "sudo", "install", "-m", "0755", remoteTmp, installDst); err != nil {
		return fmt.Errorf("deploy: install: %w", err)
	}

	// Invoke the freshly installed binary with the host sub-subcommand.
	invokeArgs := append([]string{"host"}, hostArgs...)
	fmt.Fprintf(stdout, "deploy: invoking ds-arrakis host on %s\n", host)
	if _, err := sshClient.Run(ctx, host, "sudo", append([]string{"ds-arrakis"}, invokeArgs...)...); err != nil {
		return fmt.Errorf("deploy: invoke: %w", err)
	}

	fmt.Fprintf(stdout, "deploy: done\n")
	return nil
}
