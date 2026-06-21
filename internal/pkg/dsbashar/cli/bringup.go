package cli

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path"

	"go.muehmer.eu/dapdsm/internal/pkg/dsbashar/config"
	"go.muehmer.eu/dapdsm/pkg/domain/clusterconfig"
	"go.muehmer.eu/dapdsm/pkg/domain/imageload"
	"go.muehmer.eu/dapdsm/pkg/transport/clusteraccess"
	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

// bringupDeps are the orchestration's side effects, injected for tests.
type bringupDeps struct {
	loadCluster func(ctx context.Context) (config.Config, bool, error) // (cfg, exists, err)
	listBGs     func(ctx context.Context) ([]string, error)
	promote     func(ctx context.Context, cfg config.Config, flsToken, serverPassword []byte) error
	runSetup    func(ctx context.Context, cfg config.Config, flsTokenPath string) error
	loadImg     func(ctx context.Context) error
	initDB      func(ctx context.Context, stdout, stderr io.Writer) error
	reconcile   func(ctx context.Context, stdout, stderr io.Writer) error
	readToken   func(path string) ([]byte, error)
}

// runBringup orchestrates the whole multi-node bring-up: resolve config →
// promote it to the cluster → gate on BattleGroup discovery → vendor setup.sh on
// the jumphost (fresh clusters only) → load the BG runtime image → init-db +
// reconcile. All side effects flow through bringupDeps so the orchestration is
// unit-tested without a live cluster.
func runBringup(ctx context.Context, in resolveInput, stdin *bufio.Reader, stdout, stderr io.Writer, d bringupDeps) error {
	// Pull any existing cluster config for the resolution state machine.
	if found, exists, err := d.loadCluster(ctx); err != nil {
		return fmt.Errorf("bringup: load cluster config: %w", err)
	} else if exists {
		in.Found, in.FoundExists = &found, true
	}

	res, err := resolveConfig(in, stdin, stdout)
	if err != nil {
		return err // includes errAbort with the doctor hint
	}

	token, err := d.readToken(res.FLSTokenPath)
	if err != nil {
		return fmt.Errorf("bringup: read FLS token: %w", err)
	}

	fmt.Fprintln(stdout, "bringup: promoting config to the cluster")
	if err := d.promote(ctx, res.Cfg, token, nil); err != nil {
		return fmt.Errorf("bringup: promote: %w", err)
	}

	bgs, err := d.listBGs(ctx)
	if err != nil {
		return fmt.Errorf("bringup: discover BattleGroups: %w", err)
	}
	if len(bgs) == 0 {
		fmt.Fprintln(stdout, "bringup: no BattleGroup yet — running vendor setup.sh")
		if err := d.runSetup(ctx, res.Cfg, res.FLSTokenPath); err != nil {
			return fmt.Errorf("bringup: setup.sh: %w", err)
		}
	} else {
		fmt.Fprintf(stdout, "bringup: BattleGroup %v already exists — skipping setup.sh\n", bgs)
	}

	fmt.Fprintln(stdout, "bringup: loading BG runtime image")
	if err := d.loadImg(ctx); err != nil {
		return fmt.Errorf("bringup: load BG image: %w", err)
	}

	fmt.Fprintln(stdout, "bringup: init-db")
	if err := d.initDB(ctx, stdout, stderr); err != nil {
		return fmt.Errorf("bringup: init-db: %w", err)
	}
	fmt.Fprintln(stdout, "bringup: reconcile")
	if err := d.reconcile(ctx, stdout, stderr); err != nil {
		return fmt.Errorf("bringup: reconcile: %w", err)
	}
	fmt.Fprintln(stdout, "bringup: done — BattleGroup is coming up")
	return nil
}

// bringupCmd parses flags and wires the production bringupDeps. Bring-up is
// multi-node only: it requires the global --jump flag so the orchestration can
// reach the cluster, the jumphost setup.sh, and the staged BG image.
func bringupCmd(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("bringup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "BattleGroup name (WorldName)")
	display := fs.String("display", "", "server display name")
	region := fs.String("region", "", "world region (Asia/Europe/North America/Oceania/South America)")
	fls := fs.String("fls-token", "", "path to the FLS token file on the workstation")
	noInput := fs.Bool("no-input", false, "never prompt; fail instead of running the wizard")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("bringup: %w: %w", ErrUsage, err)
	}
	if resolvedAccess == nil {
		return fmt.Errorf("bringup requires --jump (multi-node only): %w", ErrUsage)
	}
	in := resolveInput{
		Flags:        config.Override{WorldName: *name, WorldRegion: *region, ServerDisplayName: *display},
		FLSTokenFlag: *fls,
		BGNameFlag:   *name,
		NoInput:      *noInput,
	}
	deps := defaultBringupDeps(resolvedAccess, stderr)
	return runBringup(ctx, in, bufio.NewReader(stdin), stdout, stderr, deps)
}

// ccKubectl adapts a *clusteraccess.Access to clusterconfig.Kubectl.
type ccKubectl struct{ a *clusteraccess.Access }

func (k ccKubectl) Get(ctx context.Context, args ...string) ([]byte, error) {
	res, err := k.a.Kubectl(ctx, args...)
	if err != nil {
		return nil, err
	}
	return []byte(res.Stdout), nil
}

func (k ccKubectl) Apply(ctx context.Context, manifest []byte) error {
	_, err := k.a.KubectlStdin(ctx, manifest, "apply", "-f", "-")
	return err
}

// ilKubectl adapts a *clusteraccess.Access to imageload.Kubectl.
type ilKubectl struct{ a *clusteraccess.Access }

func (k ilKubectl) Run(ctx context.Context, args ...string) (string, error) {
	res, err := k.a.Kubectl(ctx, args...)
	return res.Stdout, err
}

func (k ilKubectl) Stdin(ctx context.Context, stdin []byte, args ...string) (string, error) {
	res, err := k.a.KubectlStdin(ctx, stdin, args...)
	return res.Stdout, err
}

// defaultBringupDeps wires the production side effects over the resolved Access.
//
// Live-gated: this wiring is exercised only against a real multi-node cluster
// (the unit tests drive runBringup with fakes). It must compile and be
// correct-by-construction; the jumphost paths below are confirmed at the §8 live
// test.
func defaultBringupDeps(a *clusteraccess.Access, stderr io.Writer) bringupDeps {
	store := clusterconfig.Store{KC: ccKubectl{a}, Namespace: config.ConfigNamespace}
	return bringupDeps{
		loadCluster: func(ctx context.Context) (config.Config, bool, error) {
			data, err := store.Load(ctx, config.ConfigMapName)
			if err == clusterconfig.ErrNotFound {
				return config.Config{}, false, nil
			}
			if err != nil {
				return config.Config{}, false, err
			}
			return config.FromData(data), true, nil
		},
		listBGs: func(ctx context.Context) ([]string, error) {
			return kube.ListBattleGroupNamespaces(ctx, newKubeRunner(stderr))
		},
		promote: func(ctx context.Context, cfg config.Config, flsToken, serverPassword []byte) error {
			return store.Save(ctx, config.ConfigMapName, config.ToData(cfg, flsToken, serverPassword))
		},
		runSetup: func(ctx context.Context, cfg config.Config, _ string) error {
			return runBringupSetup(ctx, a, cfg)
		},
		loadImg: func(ctx context.Context) error {
			return loadBGImage(ctx, a)
		},
		initDB: func(ctx context.Context, stdout, stderr io.Writer) error {
			return initDBCmd(ctx, nil, stdout, stderr)
		},
		reconcile: func(ctx context.Context, stdout, stderr io.Writer) error {
			return reconcileCmd(ctx, nil, stdout, stderr)
		},
		readToken: os.ReadFile,
	}
}

// setupStdin renders the stdin Funcom's setup/world.sh::main() reads:
// name → region → token (definition order in the file is misleading; the
// runtime call order matters). Mirrors setup.go's construction.
func setupStdin(cfg config.Config, regionNum int, token []byte) string {
	return fmt.Sprintf("%s\n%d\n%s\n", cfg.WorldName, regionNum, string(token))
}

// depotSetupScript is the jumphost path of Funcom's setup.sh staged under the
// depot dir for the given target. The single-node vendor path is
// vendorSetupScript (in the dune user's home); on the jumphost the script lives
// under the depot the SteamCMD slice downloaded.
//
// Live-gated: the exact path is confirmed at the §8 live test.
func depotSetupScript(target config.Target) string {
	return path.Join("/home/dune/depot", string(target), "scripts", "setup.sh")
}

// runBringupSetup drives Funcom's setup.sh on the jumphost with the bring-up
// inputs piped to stdin. Live-gated (see defaultBringupDeps).
func runBringupSetup(ctx context.Context, a *clusteraccess.Access, cfg config.Config) error {
	regionNum, err := config.RegionNumber(cfg.WorldRegion)
	if err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	// The token is materialised from the cluster Secret during promote; on the
	// jumphost setup.sh reads it from the depot env, so re-read it here from the
	// configured FLS token file path on the jumphost.
	tokenRes, err := a.OnJump(ctx, "cat", cfg.FLSTokenFile)
	if err != nil {
		return fmt.Errorf("setup: read FLS token on jumphost: %w", err)
	}
	stdin := setupStdin(cfg, regionNum, []byte(tokenRes.Stdout))
	script := depotSetupScript(cfg.Target)
	_, err = a.OnJumpStdin(ctx, []byte(stdin),
		"sudo", "-u", "dune", "env", "HOME=/home/dune", "bash", script)
	return err
}

// loadBGImage streams the staged BattleGroup runtime image into every node's
// containerd via the HTTP-fetch import path (the tar is too large for the
// kubectl-exec stream). Live-gated (see defaultBringupDeps).
func loadBGImage(ctx context.Context, a *clusteraccess.Access) error {
	// Live-gated jumphost layout; confirmed at the §8 live test.
	serveDir := "/home/dune/depot/battlegroup"
	tarPath := path.Join(serveDir, "server.tar")
	opts := imageload.HTTPOptions{
		Options: imageload.Options{
			Namespace: "ds-arrakis-imageload",
			Socket:    "/run/k3s/containerd/containerd.sock",
			CtrPath:   "/var/lib/rancher/rke2/bin/ctr",
		},
		TarPathOnJump: tarPath,
		ServeDir:      serveDir,
		ServePort:     8080,
		// JumpAddr is the pod-reachable host:port; confirmed at the live test.
		JumpAddr: "",
	}
	_, err := imageload.LoadViaHTTP(ctx, ilKubectl{a}, a, opts)
	return err
}
