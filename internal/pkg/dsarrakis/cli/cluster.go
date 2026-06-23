package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"go.muehmer.eu/dapdsm/pkg/transport/clusteraccess"
)

// clusterCmd handles `ds-arrakis cluster <verb> [flags]`. Verb "nodes" prints
// `kubectl get nodes -o wide` run on the cluster's jumphost.
func clusterCmd(ctx context.Context, ex clusteraccess.Execer, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: ds-arrakis cluster <nodes> --jump <alias> --kubeconfig <path> --inventory <path> [--distro rke2]")
		return ErrUsage
	}
	verb := args[0]
	fs := flag.NewFlagSet("cluster "+verb, flag.ContinueOnError)
	fs.SetOutput(stderr)
	jump := fs.String("jump", "", "jumphost ssh-config alias (the control host)")
	kubeconfig := fs.String("kubeconfig", "", "kubeconfig path on the jumphost")
	inventory := fs.String("inventory", "", "Ansible inventory path on the jumphost")
	distro := fs.String("distro", "rke2", "cluster distribution")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *jump == "" || *kubeconfig == "" || *inventory == "" {
		fmt.Fprintln(stderr, "cluster: --jump, --kubeconfig and --inventory are required")
		return ErrUsage
	}

	d, err := clusteraccess.Load(ctx, ex, clusteraccess.LoadParams{
		JumpHost:       *jump,
		KubeconfigPath: *kubeconfig,
		InventoryPath:  *inventory,
		Distro:         *distro,
	})
	if err != nil {
		return err
	}
	a := clusteraccess.New(ex, d)

	switch verb {
	case "nodes":
		res, err := a.Kubectl(ctx, "get", "nodes", "-o", "wide")
		if err != nil {
			if s := strings.TrimSpace(res.Stderr); s != "" {
				return fmt.Errorf("cluster nodes: %w: %s", err, s)
			}
			return fmt.Errorf("cluster nodes: %w", err)
		}
		fmt.Fprint(stdout, res.Stdout)
		return nil
	default:
		fmt.Fprintf(stderr, "cluster: unknown verb %q (want: nodes)\n", verb)
		return ErrUsage
	}
}
