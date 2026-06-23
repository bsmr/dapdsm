package cli

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
		fmt.Fprintln(stderr, "usage: ds-arrakis operators bringup --jump <alias> --kubeconfig <path> --env <prod|test> [flags]")
		return ErrUsage
	}
	fs := flag.NewFlagSet("operators bringup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jump := fs.String("jump", "", "jumphost ssh-config alias (the control host)")
	kubeconfig := fs.String("kubeconfig", "", "kubeconfig path on the jumphost")
	env := fs.String("env", "", "target environment: prod|test")
	staging := fs.String("staging", "", "depot staging dir on the jumphost (default /home/dune/depot/<env>)")
	certManagerURL := fs.String("cert-manager-url", "", "cert-manager release URL to apply; empty = skip (assume present)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *jump == "" || *kubeconfig == "" || *env == "" {
		fmt.Fprintln(stderr, "operators: --jump, --kubeconfig and --env are required")
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

	verPath := path.Join(dir, "images", "operators", "version.txt")
	verRes, err := access.OnJump(ctx, "cat", verPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", verPath, err)
	}
	version := strings.TrimSpace(verRes.Stdout)
	if version == "" {
		return fmt.Errorf("empty operator version at %s", verPath)
	}

	crdDir := path.Join(dir, "images", "operators", "crds")
	if _, err := access.OnJump(ctx, "test", "-d", crdDir); err != nil {
		return fmt.Errorf("operator CRD dir not found on jumphost: %s", crdDir)
	}

	opts := operatorbringup.Options{
		Version:        version,
		CRDDir:         crdDir,
		CertManagerURL: *certManagerURL,
	}
	fmt.Fprintf(stdout, "operators: bringing up %s (version %s)\n", *env, version)
	if err := operatorbringup.BringUp(ctx, kubectlAdapter{access}, opts); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "operators: all four operators Available")
	return nil
}
