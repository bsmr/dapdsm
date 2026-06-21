// Package kubeaccess adapts a clusteraccess.Access (kubectl executed on a
// jumphost over SSH) to the kube.Runner interface, so the existing on-VM
// ds-bashar verbs can run unchanged against a multi-node cluster.
package kubeaccess

import (
	"context"
	"fmt"

	"go.muehmer.eu/dapdsm/pkg/transport/clusteraccess"
)

// Runner satisfies kube.Runner by delegating every call to Access, which runs
// kubectl on the jumphost with KUBECONFIG exported to the descriptor path.
type Runner struct{ a *clusteraccess.Access }

// New returns a Runner bound to a resolved cluster Access.
func New(a *clusteraccess.Access) *Runner { return &Runner{a: a} }

// Get runs `kubectl get <args...>` on the jumphost and returns stdout.
// It mirrors the CmdRunner.Get convention: callers pass the resource and
// flags only — "get" is automatically prepended, so
// Get(ctx, "nodes", "-o", "json") becomes `kubectl get nodes -o json`.
func (r *Runner) Get(ctx context.Context, args ...string) ([]byte, error) {
	res, err := r.a.Kubectl(ctx, append([]string{"get"}, args...)...)
	if err != nil {
		return nil, err
	}
	return []byte(res.Stdout), nil
}

func (r *Runner) Patch(ctx context.Context, resource, name, namespace, patchType, patchJSON string) error {
	_, err := r.a.Kubectl(ctx, "patch", resource, name, "-n", namespace,
		"--type", patchType, "-p", patchJSON)
	return err
}

func (r *Runner) DeletePods(ctx context.Context, namespace string, selectors ...string) error {
	args := []string{"delete", "pod", "-n", namespace}
	for _, s := range selectors {
		args = append(args, "-l", s)
	}
	_, err := r.a.Kubectl(ctx, args...)
	return err
}

func (r *Runner) Exec(ctx context.Context, namespace, pod string, command ...string) ([]byte, error) {
	args := append([]string{"exec", pod, "-n", namespace, "--"}, command...)
	res, err := r.a.Kubectl(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("exec in %s/%s: %w", namespace, pod, err)
	}
	return []byte(res.Stdout), nil
}

func (r *Runner) ExecPiped(ctx context.Context, namespace, pod string, stdin []byte, command ...string) ([]byte, error) {
	args := append([]string{"exec", "-i", pod, "-n", namespace, "--"}, command...)
	res, err := r.a.KubectlStdin(ctx, stdin, args...)
	if err != nil {
		return nil, fmt.Errorf("exec -i in %s/%s: %w", namespace, pod, err)
	}
	return []byte(res.Stdout), nil
}
