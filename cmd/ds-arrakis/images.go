package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path"
	"strings"

	"go.muehmer.eu/dapdsm/pkg/domain/bootstrap"
	"go.muehmer.eu/dapdsm/pkg/domain/depot"
	"go.muehmer.eu/dapdsm/pkg/domain/imagedist"
	"go.muehmer.eu/dapdsm/pkg/domain/imageload"
	"go.muehmer.eu/dapdsm/pkg/transport/clusteraccess"
	"go.muehmer.eu/dapdsm/pkg/transport/skopeo"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// kubectlAdapter bridges *clusteraccess.Access to imagedist.Kubectl and
// imageload.Kubectl + imageload.Reader.
//
// Run unwraps stdout; Apply pipes a manifest to `kubectl apply -f -`;
// Stdin pipes arbitrary bytes into a kubectl subcommand;
// ReadFile fetches a file from the jumphost via `cat`.
type kubectlAdapter struct{ a *clusteraccess.Access }

func (k kubectlAdapter) Run(ctx context.Context, args ...string) (string, error) {
	res, err := k.a.Kubectl(ctx, args...)
	return res.Stdout, err
}
func (k kubectlAdapter) Apply(ctx context.Context, manifest []byte) (string, error) {
	res, err := k.a.KubectlStdin(ctx, manifest, "apply", "-f", "-")
	return res.Stdout, err
}
func (k kubectlAdapter) Stdin(ctx context.Context, stdin []byte, args ...string) (string, error) {
	res, err := k.a.KubectlStdin(ctx, stdin, args...)
	return res.Stdout, err
}

// ReadFile reads a file on the jumphost (binary-safe: ssh.Result.Stdout is a
// verbatim byte-buffer cast). Used by imageload.Reader to fetch the staged tars.
func (k kubectlAdapter) ReadFile(ctx context.Context, filePath string) ([]byte, error) {
	res, err := k.a.OnJump(ctx, "cat", filePath)
	if err != nil {
		return nil, err
	}
	return []byte(res.Stdout), nil
}

// realImagesRunner builds the production jumphost-bound skopeo.Runner.
// It reuses jumpRunner (defined in depot.go) so all SSH exec goes through
// the same adapter.
func realImagesRunner(jump string) skopeo.Runner {
	return jumpRunner{ex: ssh.NewClient(), host: jump}
}

// imagesCmd dispatches `ds-arrakis images <distribute|load> …` to the
// appropriate subcommand handler.
func imagesCmd(ctx context.Context, ex clusteraccess.Execer, newRunner func(jump string) skopeo.Runner, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: ds-arrakis images <distribute|load> [flags]")
		return ErrUsage
	}
	switch args[0] {
	case "distribute":
		return distributeCmd(ctx, ex, newRunner, args, stdout, stderr)
	case "load":
		return loadCmd(ctx, ex, args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "usage: ds-arrakis images <distribute|load> [flags]")
		return ErrUsage
	}
}

// distributeCmd handles `ds-arrakis images distribute …`.
func distributeCmd(ctx context.Context, ex clusteraccess.Execer, newRunner func(jump string) skopeo.Runner, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("images distribute", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jump := fs.String("jump", "", "jumphost ssh-config alias (the control host)")
	kubeconfig := fs.String("kubeconfig", "", "kubeconfig path on the jumphost")
	inventory := fs.String("inventory", "", "inventory path on the jumphost")
	env := fs.String("env", "", "target environment: prod|test")
	distro := fs.String("distro", "rke2", "cluster distribution")
	staging := fs.String("staging", "", "depot staging dir on the jumphost (default /home/dune/depot/<env>)")
	registry := fs.String("registry", "", "registry endpoint host:port nodes pull from (must match registries.yaml)")
	storageClass := fs.String("storage-class", "local-path", "StorageClass for the registry PVC")
	lbIP := fs.String("lb-ip", "", "LoadBalancer IP for the registry Service (the registries.yaml VIP)")
	registryImage := fs.String("registry-image", "registry:2.8.3", "registry container image")
	noVerify := fs.Bool("no-verify", false, "skip the post-push pull probe")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *jump == "" || *kubeconfig == "" || *inventory == "" || *env == "" || *registry == "" {
		fmt.Fprintln(stderr, "images: --jump, --kubeconfig, --inventory, --env and --registry are required")
		return ErrUsage
	}
	appID, err := bootstrap.AppID(*env) // validates env (prod|test)
	if err != nil {
		return err
	}
	_ = appID

	dir := *staging
	if dir == "" {
		dir = path.Join("/home/dune/depot", *env)
	}

	d, err := clusteraccess.Load(ctx, ex, clusteraccess.LoadParams{
		JumpHost: *jump, KubeconfigPath: *kubeconfig, InventoryPath: *inventory, Distro: *distro,
	})
	if err != nil {
		return err
	}
	access := clusteraccess.New(ex, d)

	// 1. deploy the in-cluster registry
	opts := imagedist.Options{
		Namespace:      "funcom-registry",
		StorageClass:   *storageClass,
		RegistryImage:  *registryImage,
		Endpoint:       *registry,
		PVCSize:        "10Gi",
		LoadBalancerIP: *lbIP,
	}
	fmt.Fprintf(stdout, "images: deploying registry %s (sc=%s)\n", *registry, *storageClass)
	if err := imagedist.Deploy(ctx, kubectlAdapter{access}, opts); err != nil {
		return err
	}

	// 2. push the depot tars into the registry
	images := path.Join(dir, "images")
	depotRes := depot.Result{
		OperatorsDir:     path.Join(images, "operators"),
		PrerequisitesDir: path.Join(images, "prerequisites"),
		BattlegroupDir:   path.Join(images, "battlegroup"),
	}
	fmt.Fprintf(stdout, "images: pushing depot tars from %s\n", images)
	res, err := imagedist.Push(ctx, newRunner(*jump), *registry, depotRes)
	if err != nil {
		return err
	}
	for _, img := range res.Images {
		fmt.Fprintf(stdout, "  pushed %s/%s:%s\n", res.Registry, img.Repo, img.Tag)
	}
	if !*noVerify {
		fmt.Fprintln(stdout, "images: (verify probe is operator-gated; run with a probe Pod at live time)")
	}
	return nil
}

// loadCmd handles `ds-arrakis images load …` — batteries-included operator-image
// load via the privileged import DaemonSet (no registry/StorageClass/registries.yaml).
// No inventory required: the DaemonSet runs on all nodes; worker discovery is done
// by listing the DaemonSet pods after rollout.
func loadCmd(ctx context.Context, ex clusteraccess.Execer, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("images load", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jump := fs.String("jump", "", "jumphost ssh-config alias (the control host)")
	kubeconfig := fs.String("kubeconfig", "", "kubeconfig path on the jumphost")
	env := fs.String("env", "", "target environment: prod|test")
	staging := fs.String("staging", "", "depot staging dir on the jumphost (default /home/dune/depot/<env>)")
	socket := fs.String("socket", "/run/k3s/containerd/containerd.sock", "host containerd socket")
	ctr := fs.String("ctr", "/var/lib/rancher/rke2/bin/ctr", "host ctr binary path")
	namespace := fs.String("namespace", "ds-arrakis-imageload", "import DaemonSet namespace")
	keep := fs.Bool("keep", false, "leave the import DaemonSet running for fast re-imports")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *jump == "" || *kubeconfig == "" || *env == "" {
		fmt.Fprintln(stderr, "images load: --jump, --kubeconfig and --env are required")
		return ErrUsage
	}
	if _, err := bootstrap.AppID(*env); err != nil { // validates env (prod|test)
		return err
	}
	dir := *staging
	if dir == "" {
		dir = path.Join("/home/dune/depot", *env)
	}

	access := clusteraccess.New(ex, &clusteraccess.Descriptor{
		JumpHost:   *jump,
		Kubeconfig: *kubeconfig,
	})

	// enumerate the operator tars on the jumphost
	opDir := path.Join(dir, "images", "operators")
	lsOut, err := access.OnJump(ctx, "sh", "-c", fmt.Sprintf("ls -1 '%s'/*.tar 2>/dev/null || true", opDir))
	if err != nil {
		return fmt.Errorf("list operator tars: %w", err)
	}
	var tars []string
	for _, line := range strings.Split(lsOut.Stdout, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			tars = append(tars, s)
		}
	}

	adapter := kubectlAdapter{access}
	fmt.Fprintf(stdout, "images load: %d operator tars -> import DaemonSet (ns %s)\n", len(tars), *namespace)
	res, err := imageload.Load(ctx, adapter, adapter, imageload.Options{
		Namespace: *namespace, Tars: tars,
		Socket: *socket, CtrPath: *ctr, KeepDaemon: *keep,
	})
	if err != nil {
		return err
	}
	for _, tar := range res.Tars {
		fmt.Fprintf(stdout, "  loaded %s into %d nodes\n", path.Base(tar), len(res.Pods))
	}
	return nil
}
