// Package crypto seals and unseals secrets with the `age` CLI (filippo.io/age
// binary), shelled out like the other transport wrappers. Model: VM-recipient
// X25519 — each VM holds an age identity; the workstation seals to the VM's
// public recipient and cannot unseal. The VM private key never leaves the VM.
package crypto

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// DefaultKeyPath is the on-VM age identity file.
const DefaultKeyPath = "/etc/dune/age.key"

// EnsureInstalled ensures `age` + `age-keygen` exist on the VM via apt-get
// (Phase 1: Ubuntu/Debian only; non-apt distro support is a later concern).
func EnsureInstalled(ctx context.Context, c *ssh.Client, host string) error {
	res, err := c.Run(ctx, host, "sh", "-c",
		"command -v age && command -v age-keygen || (sudo apt-get update && sudo apt-get install -y age)")
	if err != nil || res.ExitCode != 0 {
		return fmt.Errorf("ensure age on %s: %w (%s)", host, err, strings.TrimSpace(res.Stderr))
	}
	return nil
}

// EnsureIdentity ensures an age identity at keyPath on the VM and returns its
// public recipient. If the key is absent it is generated (0600, dune-owned).
// The private key never leaves the VM — only the recipient is returned.
func EnsureIdentity(ctx context.Context, c *ssh.Client, host, keyPath string) (string, error) {
	script := fmt.Sprintf(`set -e
if [ ! -f %[1]s ]; then
  sudo install -d -m 0700 -o dune -g dune "$(dirname %[1]s)"
  age-keygen 2>/dev/null | sudo tee %[1]s >/dev/null
  sudo chmod 0600 %[1]s
  sudo chown dune:dune %[1]s
fi
age-keygen -y %[1]s`, keyPath)
	res, err := c.Run(ctx, host, "sh", "-c", script)
	if err != nil || res.ExitCode != 0 {
		return "", fmt.Errorf("ensure age identity on %s: %w (%s)", host, err, strings.TrimSpace(res.Stderr))
	}
	// The recipient is the last age1… line of stdout.
	var recipient string
	for _, line := range strings.Split(strings.TrimSpace(res.Stdout), "\n") {
		if l := strings.TrimSpace(line); strings.HasPrefix(l, "age1") {
			recipient = l
		}
	}
	if recipient == "" {
		return "", fmt.Errorf("ensure age identity on %s: no recipient in output %q", host, res.Stdout)
	}
	return recipient, nil
}

// Seal encrypts plaintext to recipient with local `age -r`, returning the
// sealed blob. Plaintext is passed via stdin, never argv.
func Seal(ctx context.Context, runner ssh.Runner, recipient string, plaintext []byte) ([]byte, error) {
	res, err := runner.RunWithStdin(ctx, plaintext, "age", "-r", recipient)
	if err != nil || res.ExitCode != 0 {
		return nil, fmt.Errorf("age seal: %w (%s)", err, strings.TrimSpace(res.Stderr))
	}
	return []byte(res.Stdout), nil
}

// UnsealToFile decrypts sealedRemotePath on the VM into outPath (mode,
// dune-owned), using the VM identity at keyPath. Runs on the VM via ssh.
func UnsealToFile(ctx context.Context, c *ssh.Client, host, keyPath, sealedRemotePath, outPath string, mode os.FileMode) error {
	script := fmt.Sprintf(`set -e
umask 077
sudo age -d -i %s -o %s %s
sudo chmod %o %s
sudo chown dune:dune %s`, keyPath, outPath, sealedRemotePath, mode.Perm(), outPath, outPath)
	res, err := c.Run(ctx, host, "sh", "-c", script)
	if err != nil || res.ExitCode != 0 {
		return fmt.Errorf("unseal %s -> %s on %s: %w (%s)", sealedRemotePath, outPath, host, err, strings.TrimSpace(res.Stderr))
	}
	return nil
}
