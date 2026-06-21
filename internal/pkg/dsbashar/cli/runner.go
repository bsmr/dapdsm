package cli

import (
	"io"

	"go.muehmer.eu/dapdsm/pkg/transport/clusteraccess"
	"go.muehmer.eu/dapdsm/pkg/transport/kube"
	"go.muehmer.eu/dapdsm/pkg/transport/kubeaccess"
)

// kubeRunnerFor builds the kube.Runner each verb uses. Default: the local
// kubectl (single-node on-VM back-compat). configureRunner swaps it to the
// jumphost adapter when --jump is set.
//
// ponytail: process-global factory, set once at startup before any verb runs;
// the CLI is a single-shot process with no concurrency. Revisit only if Run
// ever dispatches verbs concurrently.
var kubeRunnerFor = func(stderr io.Writer) kube.Runner { return &kube.CmdRunner{Stderr: stderr} }

func newKubeRunner(stderr io.Writer) kube.Runner { return kubeRunnerFor(stderr) }

// resolvedAccess holds the *clusteraccess.Access built by configureRunner when
// --jump is set (nil otherwise). bringup needs the Access itself (not just the
// kube.Runner) to reach clusterconfig, imageload, and the jumphost setup.sh.
//
// ponytail: process-global, set once at startup like kubeRunnerFor; the CLI is a
// single-shot process with no concurrency.
var resolvedAccess *clusteraccess.Access

// configureRunner points kubeRunnerFor at the jumphost when jump != "".
func configureRunner(ex clusteraccess.Execer, jump, kubeconfig string) {
	if jump == "" {
		return
	}
	access := clusteraccess.New(ex, &clusteraccess.Descriptor{
		JumpHost:   jump,
		Kubeconfig: kubeconfig,
		Distro:     "rke2",
	})
	resolvedAccess = access
	kubeRunnerFor = func(io.Writer) kube.Runner { return kubeaccess.New(access) }
}
