package db

import (
	"context"

	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

// kubeExecer runs the in-pod command via a local kube.Runner. kube.Runner
// builds the `kubectl exec` wrapper itself, so the adapter passes the in-pod
// command straight through.
type kubeExecer struct{ r kube.Runner }

// NewKubeExecer adapts a kube.Runner to PodExecer (dunectl's on-node path).
func NewKubeExecer(r kube.Runner) PodExecer { return kubeExecer{r: r} }

func (k kubeExecer) Run(ctx context.Context, ns, pod string, stdin []byte, command ...string) (string, error) {
	if stdin == nil {
		out, err := k.r.Exec(ctx, ns, pod, command...)
		return string(out), err
	}
	out, err := k.r.ExecPiped(ctx, ns, pod, stdin, command...)
	return string(out), err
}
