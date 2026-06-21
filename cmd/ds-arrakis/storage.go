package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path"
	"strings"

	"go.muehmer.eu/dapdsm/pkg/domain/bootstrap"
	"go.muehmer.eu/dapdsm/pkg/domain/imageload"
	"go.muehmer.eu/dapdsm/pkg/domain/localpath"
	"go.muehmer.eu/dapdsm/pkg/transport/clusteraccess"
)

// storageCmd dispatches `ds-arrakis storage <subverb> …`.
func storageCmd(ctx context.Context, ex clusteraccess.Execer, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] != "local-path" {
		fmt.Fprintln(stderr, "usage: ds-arrakis storage local-path --jump <alias> --kubeconfig <path> --env <prod|test> [flags]")
		return ErrUsage
	}
	fs := flag.NewFlagSet("storage local-path", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jump := fs.String("jump", "", "jumphost ssh-config alias (the control host)")
	kubeconfig := fs.String("kubeconfig", "", "kubeconfig path on the jumphost")
	env := fs.String("env", "", "target environment: prod|test")
	staging := fs.String("staging", "", "depot staging dir on the jumphost (default /home/dune/depot/<env>)")
	socket := fs.String("socket", "/run/k3s/containerd/containerd.sock", "host containerd socket")
	ctr := fs.String("ctr", "/var/lib/rancher/rke2/bin/ctr", "host ctr binary path")
	namespace := fs.String("namespace", "ds-arrakis-imageload", "import DaemonSet namespace")
	keep := fs.Bool("keep", false, "leave the import DaemonSet running for fast re-imports")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *jump == "" || *kubeconfig == "" || *env == "" {
		fmt.Fprintln(stderr, "storage: --jump, --kubeconfig and --env are required")
		return ErrUsage
	}
	if _, err := bootstrap.AppID(*env); err != nil {
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

	// Enumerate the provisioner tar(s) under images/prerequisites/ on the jumphost.
	prereqDir := path.Join(dir, "images", "prerequisites")
	lsOut, err := access.OnJump(ctx, "sh", "-c",
		fmt.Sprintf("ls -1 '%s'/local-path-provisioner*.tar 2>/dev/null || true", prereqDir))
	if err != nil {
		return fmt.Errorf("list provisioner tars: %w", err)
	}
	var tars []string
	for _, line := range strings.Split(lsOut.Stdout, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			tars = append(tars, s)
		}
	}
	if len(tars) == 0 {
		return fmt.Errorf("no local-path-provisioner*.tar found under %s", prereqDir)
	}

	adapter := kubectlAdapter{access}

	// load loads the provisioner image via the DaemonSet import path (same
	// mechanism as `images load`; reuses the same seam).
	load := func(ctx context.Context) error {
		fmt.Fprintf(stdout, "storage local-path: loading %d provisioner tar(s)\n", len(tars))
		res, err := imageload.Load(ctx, adapter, adapter, imageload.Options{
			Namespace:  *namespace,
			Tars:       tars,
			Socket:     *socket,
			CtrPath:    *ctr,
			KeepDaemon: *keep,
		})
		if err != nil {
			return err
		}
		for _, tar := range res.Tars {
			fmt.Fprintf(stdout, "  loaded %s into %d nodes\n", path.Base(tar), len(res.Pods))
		}
		return nil
	}

	fmt.Fprintf(stdout, "storage local-path: installing StorageClass on %s\n", *env)
	if err := localpath.Install(ctx, adapter, load, localpath.Options{DepotDir: dir}); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "storage local-path: local-path StorageClass present")
	return nil
}
