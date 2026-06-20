package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path"
	"strings"

	"go.muehmer.eu/dapdsm/pkg/domain/bootstrap"
	"go.muehmer.eu/dapdsm/pkg/domain/operatorbringup"
	"go.muehmer.eu/dapdsm/pkg/transport/clusteraccess"
)

// operatorsCmd handles `ds-arrakis operators bringup …`.
func operatorsCmd(ctx context.Context, ex clusteraccess.Execer, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] != "bringup" {
		fmt.Fprintln(stderr, "usage: ds-arrakis operators bringup --jump <alias> --kubeconfig <path> --inventory <path> --env <prod|test> --registry <endpoint> [flags]")
		return ErrUsage
	}
	fs := flag.NewFlagSet("operators bringup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jump := fs.String("jump", "", "jumphost ssh-config alias (the control host)")
	kubeconfig := fs.String("kubeconfig", "", "kubeconfig path on the jumphost")
	inventory := fs.String("inventory", "", "inventory path on the jumphost")
	env := fs.String("env", "", "target environment: prod|test")
	registry := fs.String("registry", "", "S2 registry endpoint host:port (image-ref target)")
	distro := fs.String("distro", "rke2", "cluster distribution")
	staging := fs.String("staging", "", "depot staging dir on the jumphost (default /home/dune/depot/<env>)")
	certManagerURL := fs.String("cert-manager-url", "", "cert-manager release URL to apply; empty = skip (assume present)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *jump == "" || *kubeconfig == "" || *inventory == "" || *env == "" || *registry == "" {
		fmt.Fprintln(stderr, "operators: --jump, --kubeconfig, --inventory, --env and --registry are required")
		return ErrUsage
	}
	if _, err := bootstrap.AppID(*env); err != nil {
		return err
	}
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

	verPath := path.Join(dir, "images", "operators", "version.txt")
	verRes, err := access.OnJump(ctx, "cat", verPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", verPath, err)
	}
	version := strings.TrimSpace(verRes.Stdout)
	if version == "" {
		return fmt.Errorf("empty operator version at %s", verPath)
	}

	var workers []string
	for _, n := range access.Nodes(clusteraccess.RoleWorker) {
		workers = append(workers, n.Name)
	}

	crdDir := path.Join(dir, "images", "operators", "crds")
	if _, err := access.OnJump(ctx, "test", "-d", crdDir); err != nil {
		return fmt.Errorf("operator CRD dir not found on jumphost: %s", crdDir)
	}

	opts := operatorbringup.Options{
		Registry:       *registry,
		Version:        version,
		CRDDir:         crdDir,
		CertManagerURL: *certManagerURL,
		Workers:        workers,
	}
	fmt.Fprintf(stdout, "operators: bringing up %s (version %s, registry %s, %d workers)\n",
		*env, version, *registry, len(workers))
	if err := operatorbringup.BringUp(ctx, kubectlAdapter{access}, opts); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "operators: all four operators Available")
	return nil
}
