package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"go.muehmer.eu/dapdsm/internal/pkg/dsbashar/config"
	"go.muehmer.eu/dapdsm/pkg/domain/battlegroup"
	"go.muehmer.eu/dapdsm/pkg/domain/clusterconfig"
	"go.muehmer.eu/dapdsm/pkg/domain/imageload"
	"go.muehmer.eu/dapdsm/pkg/domain/worldsetup"
	"go.muehmer.eu/dapdsm/pkg/transport/clusteraccess"
	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

// bringupDeps are the orchestration's side effects, injected for tests.
type bringupDeps struct {
	loadCluster        func(ctx context.Context) (config.Config, bool, error) // (cfg, exists, err)
	verifyStorageClass func(ctx context.Context) error
	listBGs            func(ctx context.Context) ([]string, error)
	promote            func(ctx context.Context, cfg config.Config, flsToken, serverPassword []byte) error
	runSetup           func(ctx context.Context, cfg config.Config, flsToken []byte) error
	loadImg            func(ctx context.Context, target config.Target) error
	reconcileImageTags func(ctx context.Context, target config.Target) error
	initDB             func(ctx context.Context, stdout, stderr io.Writer) error
	reconcile          func(ctx context.Context, stdout, stderr io.Writer) error
	readToken          func(path string) ([]byte, error)
}

// runBringup orchestrates the whole multi-node bring-up: resolve config →
// verify StorageClass → promote config to the cluster → gate on BattleGroup
// discovery → worldsetup (fresh clusters only) → load the BG runtime image →
// reconcile image tags → init-db + reconcile. All side effects flow through
// bringupDeps so the orchestration is unit-tested without a live cluster.
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

	fmt.Fprintln(stdout, "bringup: verifying StorageClass")
	if err := d.verifyStorageClass(ctx); err != nil {
		return fmt.Errorf("bringup: %w", err)
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
		fmt.Fprintln(stdout, "bringup: no BattleGroup yet — running worldsetup")
		if err := d.runSetup(ctx, res.Cfg, token); err != nil {
			return fmt.Errorf("bringup: worldsetup: %w", err)
		}
	} else {
		fmt.Fprintf(stdout, "bringup: BattleGroup %v already exists — skipping worldsetup\n", bgs)
	}

	fmt.Fprintln(stdout, "bringup: loading BG runtime image")
	if err := d.loadImg(ctx, res.Cfg.Target); err != nil {
		return fmt.Errorf("bringup: load BG image: %w", err)
	}

	fmt.Fprintln(stdout, "bringup: reconciling BattleGroup image tags")
	if err := d.reconcileImageTags(ctx, res.Cfg.Target); err != nil {
		return fmt.Errorf("bringup: reconcile image tags: %w", err)
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
// reach the cluster, the jumphost worldsetup, and the staged BG image.
func bringupCmd(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("bringup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "BattleGroup name (WorldName)")
	display := fs.String("display", "", "server display name")
	region := fs.String("region", "", "world region (Asia/Europe/North America/Oceania/South America)")
	fls := fs.String("fls-token", "", "path to the FLS token file on the workstation")
	noInput := fs.Bool("no-input", false, "never prompt; fail instead of running the wizard")
	jumpAddr := fs.String("jump-addr", "", "pod-reachable jumphost host:port for image fetch (e.g. 192.168.13.5:8080)")
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
	deps := defaultBringupDeps(resolvedAccess, *jumpAddr, stderr)
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

// wsSeam adapts *clusteraccess.Access to worldsetup.Seam.
type wsSeam struct{ a *clusteraccess.Access }

func (s wsSeam) ReadDepotFile(ctx context.Context, p string) ([]byte, error) {
	res, err := s.a.OnJump(ctx, "cat", p)
	if err != nil {
		return nil, err
	}
	return []byte(res.Stdout), nil
}

func (s wsSeam) Kubectl(ctx context.Context, args ...string) (string, error) {
	res, err := s.a.Kubectl(ctx, args...)
	return res.Stdout, err
}

func (s wsSeam) KubectlStdin(ctx context.Context, in []byte, args ...string) (string, error) {
	res, err := s.a.KubectlStdin(ctx, in, args...)
	return res.Stdout, err
}

// firstSeabassNamespace returns the first funcom-seabass-* namespace found on
// the cluster. Returns an error if none is found.
func firstSeabassNamespace(ctx context.Context, a *clusteraccess.Access) (string, error) {
	res, err := a.Kubectl(ctx, "get", "ns", "-o", "name")
	if err != nil {
		return "", fmt.Errorf("list namespaces: %w", err)
	}
	for _, line := range strings.Split(res.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "funcom-seabass-") {
			// kubectl outputs "namespace/funcom-seabass-xxx"; strip the prefix.
			ns := strings.TrimPrefix(line, "namespace/")
			return ns, nil
		}
	}
	return "", fmt.Errorf("no funcom-seabass-* namespace found on the cluster")
}

// bgImageTag reads the BattleGroup image tag from the depot's version.txt on
// the jumphost. The exact tag format (<revision>-0-shipping) is confirmed at
// the §8 live test. // live-gated
func bgImageTag(ctx context.Context, a *clusteraccess.Access, target config.Target) (string, error) {
	// live-gated: path and tag format confirmed at the §8 live test.
	versionPath := path.Join("/home/dune/depot", string(target), "images/battlegroup/version.txt")
	res, err := a.OnJump(ctx, "cat", versionPath)
	if err != nil {
		return "", fmt.Errorf("read battlegroup version.txt: %w", err)
	}
	tag := strings.TrimSpace(res.Stdout)
	if tag == "" {
		return "", fmt.Errorf("battlegroup version.txt is empty at %s", versionPath)
	}
	return tag, nil
}

// defaultBringupDeps wires the production side effects over the resolved Access.
//
// Live-gated: this wiring is exercised only against a real multi-node cluster
// (the unit tests drive runBringup with fakes). It must compile and be
// correct-by-construction; the jumphost paths below are confirmed at the §8 live
// test.
func defaultBringupDeps(a *clusteraccess.Access, jumpAddr string, stderr io.Writer) bringupDeps {
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
		verifyStorageClass: func(ctx context.Context) error {
			out, err := a.Kubectl(ctx, "get", "storageclass", "-o", "name")
			if err != nil {
				return fmt.Errorf("query StorageClasses: %w", err)
			}
			if strings.TrimSpace(out.Stdout) == "" {
				return fmt.Errorf("no StorageClass on the cluster — run `ds-arrakis storage local-path` first")
			}
			return nil
		},
		listBGs: func(ctx context.Context) ([]string, error) {
			return kube.ListBattleGroupNamespaces(ctx, newKubeRunner(stderr))
		},
		promote: func(ctx context.Context, cfg config.Config, flsToken, serverPassword []byte) error {
			return store.Save(ctx, config.ConfigMapName, config.ToData(cfg, flsToken, serverPassword))
		},
		runSetup: func(ctx context.Context, cfg config.Config, flsToken []byte) error {
			_, err := worldsetup.CreateWorld(ctx, wsSeam{a}, worldsetup.Config{
				WorldName:   cfg.WorldName,
				WorldRegion: cfg.WorldRegion,
				DepotDir:    path.Join("/home/dune/depot", string(cfg.Target)), // live-gated
			}, flsToken)
			return err
		},
		loadImg: func(ctx context.Context, target config.Target) error {
			return loadBGImage(ctx, a, jumpAddr, target)
		},
		reconcileImageTags: func(ctx context.Context, target config.Target) error {
			ns, err := firstSeabassNamespace(ctx, a)
			if err != nil {
				return err
			}
			bg := kube.BattleGroupName(ns)
			crRes, err := a.Kubectl(ctx, "get", "battlegroup", bg, "-n", ns, "-o", "json")
			if err != nil {
				return err
			}
			tag, err := bgImageTag(ctx, a, target)
			if err != nil {
				return err
			}
			ops, err := battlegroup.BuildImageTagPatches([]byte(crRes.Stdout), tag)
			if err != nil {
				return fmt.Errorf("build image-tag patches: %w", err)
			}
			if len(ops) == 0 {
				return nil
			}
			payload, err := json.Marshal(ops)
			if err != nil {
				return err
			}
			_, err = a.KubectlStdin(ctx, nil, "patch", "battlegroup", bg, "-n", ns,
				"--type=json", "-p", string(payload))
			return err
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

// loadBGImage streams the staged BattleGroup runtime images into every node's
// containerd via the HTTP-fetch import path (the tars are too large for the
// kubectl-exec stream). Two LoadViaHTTP calls are required because ServeDir
// must root at a single directory and the BG tars and igw-postgres live in
// different depot subdirectories. Live-gated (see defaultBringupDeps).
func loadBGImage(ctx context.Context, a *clusteraccess.Access, jumpAddr string, target config.Target) error {
	depotDir := path.Join("/home/dune/depot", string(target)) // live-gated

	// Enumerate battlegroup tars dynamically — filenames are not known without
	// the live depot. Mirror the ls-glob pattern from cmd/ds-arrakis/images.go.
	bgDir := path.Join(depotDir, "images", "battlegroup") // live-gated
	lsRes, err := a.OnJump(ctx, "sh", "-c", fmt.Sprintf("ls -1 '%s'/*.tar 2>/dev/null || true", bgDir))
	if err != nil {
		return fmt.Errorf("list battlegroup tars: %w", err)
	}
	var bgTars []string
	for _, line := range strings.Split(lsRes.Stdout, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			bgTars = append(bgTars, s)
		}
	}

	commonOpts := imageload.Options{
		Namespace: "ds-arrakis-imageload",
		Socket:    "/run/k3s/containerd/containerd.sock", // live-gated
		CtrPath:   "/var/lib/rancher/rke2/bin/ctr",       // live-gated
	}

	// First call: all battlegroup tars served from bgDir.
	_, err = imageload.LoadViaHTTP(ctx, ilKubectl{a}, a, imageload.HTTPOptions{
		Options:   commonOpts,
		TarPaths:  bgTars,
		ServeDir:  bgDir,
		ServePort: 8080,
		JumpAddr:  jumpAddr,
	})
	if err != nil {
		return fmt.Errorf("load battlegroup tars: %w", err)
	}

	// Second call: igw-postgres lives under images/prerequisites, which is a
	// different ServeDir and therefore requires a separate LoadViaHTTP call.
	prereqDir := path.Join(depotDir, "images", "prerequisites") // live-gated
	_, err = imageload.LoadViaHTTP(ctx, ilKubectl{a}, a, imageload.HTTPOptions{
		Options:   commonOpts,
		TarPaths:  []string{path.Join(prereqDir, "igw-postgres.tar")},
		ServeDir:  prereqDir,
		ServePort: 8080,
		JumpAddr:  jumpAddr,
	})
	if err != nil {
		return fmt.Errorf("load igw-postgres tar: %w", err)
	}
	return nil
}
