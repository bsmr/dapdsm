package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path"

	"go.muehmer.eu/dapdsm/pkg/domain/bootstrap"
	"go.muehmer.eu/dapdsm/pkg/domain/depot"
	"go.muehmer.eu/dapdsm/pkg/domain/imagedist"
	"go.muehmer.eu/dapdsm/pkg/transport/clusteraccess"
	"go.muehmer.eu/dapdsm/pkg/transport/skopeo"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// kubectlAdapter bridges *clusteraccess.Access to imagedist.Kubectl: Run
// unwraps stdout; Apply pipes a manifest to `kubectl apply -f -`.
type kubectlAdapter struct{ a *clusteraccess.Access }

func (k kubectlAdapter) Run(ctx context.Context, args ...string) (string, error) {
	res, err := k.a.Kubectl(ctx, args...)
	return res.Stdout, err
}
func (k kubectlAdapter) Apply(ctx context.Context, manifest []byte) (string, error) {
	res, err := k.a.KubectlStdin(ctx, manifest, "apply", "-f", "-")
	return res.Stdout, err
}

// realImagesRunner builds the production jumphost-bound skopeo.Runner.
// It reuses jumpRunner (defined in depot.go) so all SSH exec goes through
// the same adapter.
func realImagesRunner(jump string) skopeo.Runner {
	return jumpRunner{ex: ssh.NewClient(), host: jump}
}

// imagesCmd handles `ds-arrakis images distribute …`.
func imagesCmd(ctx context.Context, ex clusteraccess.Execer, newRunner func(jump string) skopeo.Runner, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] != "distribute" {
		fmt.Fprintln(stderr, "usage: ds-arrakis images distribute --jump <alias> --kubeconfig <path> --inventory <path> --env <prod|test> [flags]")
		return ErrUsage
	}
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
