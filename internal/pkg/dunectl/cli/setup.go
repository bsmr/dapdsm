package cli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"

	"go.muehmer.eu/dapdsm/internal/pkg/dunectl/config"
)

// setupRunFunc is the indirection that lets tests inject a fake exec.
// Production wires it to a thin wrapper around exec.CommandContext that
// pipes stdin and forwards stdout/stderr.
type setupRunFunc func(ctx context.Context, stdin []byte, name string, args ...string) ([]byte, error)

type setupDeps struct {
	cfg      config.Config
	run      setupRunFunc
	tokenSrc func() ([]byte, error)
}

// vendorSetupScript is the on-disk path of Funcom's setup.sh inside the
// dune user's home — populated by SteamCMD on bootstrap.
const vendorSetupScript = "/home/dune/.dune/download/scripts/setup.sh"

func setupCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	cfg, err := config.LoadFromFile(config.DefaultPath)
	if err != nil {
		return err
	}
	deps := setupDeps{
		cfg:      cfg,
		run:      defaultSetupRun(stdout, stderr),
		tokenSrc: func() ([]byte, error) { return os.ReadFile(cfg.FLSTokenFile) },
	}
	return runSetup(ctx, args, stdout, stderr, deps)
}

// defaultSetupRun shells out via `sudo -u dune env HOME=/home/dune bash …`,
// pipes the supplied stdin, and forwards stdout/stderr to the writers so
// the operator sees Funcom's setup output in real time.
func defaultSetupRun(stdout, stderr io.Writer) setupRunFunc {
	return func(ctx context.Context, stdin []byte, name string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Stdin = bytes.NewReader(stdin)
		var out bytes.Buffer
		cmd.Stdout = io.MultiWriter(&out, stdout)
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			return out.Bytes(), fmt.Errorf("%s %v: %w", name, args, err)
		}
		return out.Bytes(), nil
	}
}

// runSetup drives Funcom's vendor setup.sh non-interactively. It reads
// WorldName / WorldRegion from /etc/dune/dunectl.env (validated against
// YAML-safety and Funcom's region list), reads the FLS token from
// FLSTokenFile, and pipes "name\nregion\ntoken\n" to setup.sh in the
// order setup/world.sh::main() expects them.
//
// Idempotency is provided by Funcom's setup.sh itself: re-running on an
// already-bootstrapped BG is not safe (it tries to recreate the
// namespace). dunectl setup is meant for fresh BattleGroups only.
func runSetup(ctx context.Context, args []string, stdout, stderr io.Writer, deps setupDeps) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: dunectl setup\n\n"+
			"Reads WORLD_NAME and WORLD_REGION from /etc/dune/dunectl.env,\n"+
			"reads the FLS token from FLS_TOKEN_FILE, and pipes them to\n"+
			"Funcom's setup.sh in non-interactive mode.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("setup: %w: %w", ErrUsage, err)
	}

	if deps.cfg.WorldName == "" {
		return fmt.Errorf("setup: WORLD_NAME not set in /etc/dune/dunectl.env")
	}
	if !config.ValidWorldName(deps.cfg.WorldName) {
		return fmt.Errorf("setup: WORLD_NAME %q is not YAML-safe (allowed: [A-Za-z0-9_-])",
			deps.cfg.WorldName)
	}
	if deps.cfg.WorldRegion == "" {
		return fmt.Errorf("setup: WORLD_REGION not set in /etc/dune/dunectl.env")
	}
	regionNum, err := config.RegionNumber(deps.cfg.WorldRegion)
	if err != nil {
		return fmt.Errorf("setup: %w", err)
	}

	token, err := deps.tokenSrc()
	if err != nil {
		return fmt.Errorf("setup: read FLS token: %w", err)
	}

	// setup/world.sh::main() reads name → region → token (definition
	// order in the file is misleading; the runtime call order matters).
	stdin := fmt.Sprintf("%s\n%d\n%s\n", deps.cfg.WorldName, regionNum, string(token))

	fmt.Fprintf(stdout, "setup: running Funcom setup.sh for BattleGroup %q in region %s (#%d)\n",
		deps.cfg.WorldName, deps.cfg.WorldRegion, regionNum)
	_, err = deps.run(ctx, []byte(stdin),
		"sudo", "-u", "dune", "env", "HOME=/home/dune",
		"bash", vendorSetupScript,
	)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "setup: done — BG namespace and CR generated\n")
	return nil
}
