// Package rollout orchestrates a workstation->VM rollout of suite binaries and
// sealed secrets for one target. It is additive: it installs binaries, delivers
// unsealed secret files, and syncs the etc/ templates, but never touches the
// operator-owned /etc/dune/dunectl.env or the K3s/RKE2 config.
package rollout

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.muehmer.eu/dapdsm/pkg/config"
	"go.muehmer.eu/dapdsm/pkg/transport/crypto"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// SecretDest is the on-VM destination for an unsealed secret.
type SecretDest struct {
	Path string
	Mode os.FileMode
}

// secretDest resolves the on-VM plaintext destination for a secret key given
// the rollout target. The fls-token destination is target-kind-specific
// (/etc/dune/fls-token-prod or /etc/dune/fls-token-test) to match the path
// ds-bashar reads by default; Kind must therefore be set on the target.
func secretDest(key string, t *config.Target) (SecretDest, error) {
	switch key {
	case "fls-token":
		if t.Kind == "" {
			return SecretDest{}, fmt.Errorf("secret fls-token requires target kind (prod|test)")
		}
		return SecretDest{Path: "/etc/dune/fls-token-" + t.Kind, Mode: 0o600}, nil
	case "server-password":
		return SecretDest{Path: "/etc/dune/server-password", Mode: 0o600}, nil
	default:
		return SecretDest{}, fmt.Errorf("no on-VM destination for secret %q", key)
	}
}

// Deps are the injectable collaborators (fakes in tests).
type Deps struct {
	SSH        *ssh.Client
	Build      func(ctx context.Context, binary string) (localPath string, err error)
	EtcArchive func(ctx context.Context) ([]byte, error)
	Stdout     io.Writer
}

// Run rolls out targetName: build+install binaries, deliver+unseal secrets, sync etc/.
func Run(ctx context.Context, deps Deps, cfg *config.Config, configDir, targetName string) error {
	t, ok := cfg.Target(targetName)
	if !ok {
		return fmt.Errorf("rollout: unknown target %q", targetName)
	}

	if err := crypto.EnsureInstalled(ctx, deps.SSH, t.Name); err != nil {
		return fmt.Errorf("rollout %s: %w", t.Name, err)
	}
	recipient, err := crypto.EnsureIdentity(ctx, deps.SSH, t.Name, crypto.DefaultKeyPath)
	if err != nil {
		return fmt.Errorf("rollout %s: %w", t.Name, err)
	}
	if t.Recipient == "" {
		return fmt.Errorf("rollout %s: target has no cached recipient — run `ds-thumper init %s` first", t.Name, t.Name)
	}
	if recipient != t.Recipient {
		return fmt.Errorf("rollout %s: VM recipient %q != cached %q (identity changed?)", t.Name, recipient, t.Recipient)
	}

	for _, bin := range t.Binaries {
		local, err := deps.Build(ctx, bin)
		if err != nil {
			return fmt.Errorf("rollout %s: build %s: %w", t.Name, bin, err)
		}
		tmp := "/tmp/" + bin
		if err := deps.SSH.SendFile(ctx, t.Name, local, tmp); err != nil {
			return fmt.Errorf("rollout %s: push %s: %w", t.Name, bin, err)
		}
		if _, err := deps.SSH.Run(ctx, t.Name, "sh", "-c",
			fmt.Sprintf("sudo install -m 0755 %s /usr/local/bin/%s && rm -f %s", tmp, bin, tmp)); err != nil {
			return fmt.Errorf("rollout %s: install %s: %w", t.Name, bin, err)
		}
		fmt.Fprintf(deps.Stdout, "installed %s on %s\n", bin, t.Name)
	}

	for key, rel := range t.Secrets {
		dest, err := secretDest(key, t)
		if err != nil {
			return fmt.Errorf("rollout %s: %w", t.Name, err)
		}
		local := filepath.Join(configDir, rel)
		if _, err := os.Stat(local); err != nil {
			return fmt.Errorf("rollout %s: sealed secret %s: %w", t.Name, key, err)
		}
		remoteSealed := "/tmp/" + key + ".age"
		if err := deps.SSH.SendFile(ctx, t.Name, local, remoteSealed); err != nil {
			return fmt.Errorf("rollout %s: push secret %s: %w", t.Name, key, err)
		}
		if err := crypto.UnsealToFile(ctx, deps.SSH, t.Name, crypto.DefaultKeyPath, remoteSealed, dest.Path, dest.Mode); err != nil {
			return fmt.Errorf("rollout %s: unseal %s: %w", t.Name, key, err)
		}
		_, _ = deps.SSH.Run(ctx, t.Name, "rm", "-f", remoteSealed)
		fmt.Fprintf(deps.Stdout, "delivered secret %s on %s\n", key, t.Name)
	}

	tarBytes, err := deps.EtcArchive(ctx)
	if err != nil {
		return fmt.Errorf("rollout %s: archive etc: %w", t.Name, err)
	}
	if len(tarBytes) > 0 {
		if _, err := deps.SSH.RunWithStdin(ctx, t.Name, tarBytes, "sh", "-c",
			"sudo install -d -m 0755 /opt/dapdsm && sudo tar -x -C /opt/dapdsm && sudo chown -R root:root /opt/dapdsm"); err != nil {
			return fmt.Errorf("rollout %s: sync etc: %w", t.Name, err)
		}
		fmt.Fprintf(deps.Stdout, "synced etc/ to %s:/opt/dapdsm\n", t.Name)
	}
	return nil
}
