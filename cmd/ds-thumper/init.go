package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.muehmer.eu/dapdsm/pkg/config"
	"go.muehmer.eu/dapdsm/pkg/transport/crypto"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

func initCmd(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "init: expected exactly one <host>")
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

	client := ssh.NewClient()
	if err := crypto.EnsureInstalled(ctx, client, host); err != nil {
		return err
	}
	recipient, err := crypto.EnsureIdentity(ctx, client, host, crypto.DefaultKeyPath)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "VM %s age recipient: %s\n", host, recipient)

	tgt, err := wizardTarget(stdin, stdout, host, recipient)
	if err != nil {
		return err
	}

	// Seal secrets interactively (plaintext via stdin, sealed file written, plaintext discarded).
	if err := sealSecrets(ctx, stdin, stdout, client.Runner, dir, &tgt); err != nil {
		return err
	}

	// Upsert the target.
	replaced := false
	for i := range cfg.Targets {
		if cfg.Targets[i].Name == host {
			cfg.Targets[i] = tgt
			replaced = true
		}
	}
	if !replaced {
		cfg.Targets = append(cfg.Targets, tgt)
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := cfg.Save(dir); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "saved target %s to %s/config.yaml\n", host, dir)
	return nil
}

// wizardTarget gathers the non-secret target config from prompts.
func wizardTarget(stdin io.Reader, stdout io.Writer, host, recipient string) (config.Target, error) {
	r := bufio.NewReader(stdin)

	fmt.Fprintf(stdout, "Target kind (prod/test): ")
	kindLine, _ := r.ReadString('\n')
	kind := strings.TrimSpace(kindLine)
	if kind != "prod" && kind != "test" {
		return config.Target{}, fmt.Errorf("invalid target kind %q (must be prod or test)", kind)
	}

	fmt.Fprintf(stdout, "Binaries to roll out (space-separated, from %v) [ds-bashar]: ", keys(config.KnownBinaries))
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		line = "ds-bashar"
	}
	var bins []string
	for _, b := range strings.Fields(line) {
		if !config.KnownBinaries[b] {
			return config.Target{}, fmt.Errorf("unknown binary %q (known: %v)", b, keys(config.KnownBinaries))
		}
		bins = append(bins, b)
	}
	return config.Target{Name: host, Recipient: recipient, Kind: kind, Binaries: bins, Secrets: map[string]string{}}, nil
}

// sealSecrets prompts per known secret key; on "y" reads the secret (one line),
// seals it to the recipient, writes secrets/<host>/<key>.age, records the ref.
// Keys are iterated in sorted order for a stable prompt sequence.
func sealSecrets(ctx context.Context, stdin io.Reader, stdout io.Writer, runner ssh.Runner, dir string, tgt *config.Target) error {
	r := bufio.NewReader(stdin)
	if runner == nil {
		runner = ssh.NewRunner()
	}
	secretKeys := make([]string, 0, len(config.KnownSecretKeys))
	for k := range config.KnownSecretKeys {
		secretKeys = append(secretKeys, k)
	}
	sort.Strings(secretKeys)
	for _, key := range secretKeys {
		fmt.Fprintf(stdout, "Seal secret %q now? [y/N]: ", key)
		yn, _ := r.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(yn)) != "y" {
			continue
		}
		fmt.Fprintf(stdout, "  enter %s value: ", key)
		val, _ := r.ReadString('\n')
		val = strings.TrimRight(val, "\r\n")
		sealed, err := crypto.Seal(ctx, runner, tgt.Recipient, []byte(val))
		if err != nil {
			return err
		}
		rel := filepath.Join("secrets", tgt.Name, key+".age")
		abs := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(abs, sealed, 0o600); err != nil {
			return err
		}
		tgt.Secrets[key] = rel
		fmt.Fprintf(stdout, "  sealed -> %s\n", rel)
	}
	return nil
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
