// Package kube wraps the kubectl(1) binary so dunectl subcommands can
// talk to a Kubernetes cluster without dragging the client-go dependency
// tree into this module.
//
// kubeconfig resolution is inherited from kubectl itself: the KUBECONFIG
// environment variable, then ~/.kube/config. Pure exec; no in-process
// schema awareness.
package kube

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Getter is the read-only subset of Runner. Helpers that only read CRs
// accept this so any kubectl transport (local CmdRunner or an SSH-backed
// getter) can satisfy them.
type Getter interface {
	Get(ctx context.Context, args ...string) ([]byte, error)
}

// Runner is the subset of kubectl(1) that dunectl needs. It exists so
// tests can supply an in-memory fake instead of shelling out.
type Runner interface {
	Get(ctx context.Context, args ...string) ([]byte, error)
	Patch(ctx context.Context, resource, name, namespace, patchType, patchJSON string) error
	DeletePods(ctx context.Context, namespace string, selectors ...string) error
	// Exec runs `kubectl exec -n <namespace> <pod> -- <command...>` and
	// returns the command's stdout. Stderr is forwarded to the runner's
	// Stderr sink (if any) so diagnostics still surface during test failures.
	Exec(ctx context.Context, namespace, pod string, command ...string) ([]byte, error)
	// ExecPiped is like Exec but feeds stdin to the remote command, which
	// is required for psql workflows that use \gexec or multi-statement
	// scripts (the -c flag is single-statement only).
	ExecPiped(ctx context.Context, namespace, pod string, stdin []byte, command ...string) ([]byte, error)
}

// CmdRunner is the default Runner; it shells out to the kubectl binary.
type CmdRunner struct {
	// Bin overrides the binary name (default: "kubectl").
	Bin string
	// Kubeconfig, when non-empty, is passed as --kubeconfig=<path> on
	// every invocation. Empty inherits kubectl's own resolution
	// (KUBECONFIG env, then ~/.kube/config) — the on-node case, where
	// kubectl resolves the node's own k3s kubeconfig.
	Kubeconfig string
	// Stderr receives the kubectl command's stderr stream. nil discards.
	Stderr io.Writer
}

func (c *CmdRunner) kubectl() string {
	if c.Bin == "" {
		return "kubectl"
	}
	return c.Bin
}

// globalArgs returns the leading args common to every invocation.
func (c *CmdRunner) globalArgs() []string {
	if c.Kubeconfig != "" {
		return []string{"--kubeconfig=" + c.Kubeconfig}
	}
	return nil
}

// Get runs `kubectl get <args...>` and returns stdout.
func (c *CmdRunner) Get(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.kubectl(), append(c.globalArgs(), append([]string{"get"}, args...)...)...)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if c.Stderr != nil {
			_, _ = io.Copy(c.Stderr, &errBuf)
		}
		return nil, fmt.Errorf("kubectl get %s: %w", strings.Join(args, " "), err)
	}
	return out.Bytes(), nil
}

// Patch applies a JSON-encoded patch to the named resource.
func (c *CmdRunner) Patch(ctx context.Context, resource, name, namespace, patchType, patchJSON string) error {
	args := append(c.globalArgs(), "patch", resource, name, "-n", namespace, "--type", patchType, "-p", patchJSON)
	cmd := exec.CommandContext(ctx, c.kubectl(), args...)
	cmd.Stdout = c.Stderr
	cmd.Stderr = c.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl patch %s/%s: %w", resource, name, err)
	}
	return nil
}

// Exec runs `kubectl exec -n <namespace> <pod> -- <command...>` and returns stdout.
func (c *CmdRunner) Exec(ctx context.Context, namespace, pod string, command ...string) ([]byte, error) {
	args := append(c.globalArgs(), append([]string{"exec", "-n", namespace, pod, "--"}, command...)...)
	cmd := exec.CommandContext(ctx, c.kubectl(), args...)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if c.Stderr != nil {
			_, _ = io.Copy(c.Stderr, &errBuf)
		}
		return nil, fmt.Errorf("kubectl exec %s/%s: %w", namespace, pod, err)
	}
	return out.Bytes(), nil
}

// ExecPiped runs `kubectl exec -i -n <namespace> <pod> -- <command...>`
// with the given stdin and returns stdout. The -i flag keeps stdin open
// so multi-statement psql input (with \gexec) actually reaches the remote.
func (c *CmdRunner) ExecPiped(ctx context.Context, namespace, pod string, stdin []byte, command ...string) ([]byte, error) {
	args := append(c.globalArgs(), append([]string{"exec", "-i", "-n", namespace, pod, "--"}, command...)...)
	cmd := exec.CommandContext(ctx, c.kubectl(), args...)
	cmd.Stdin = bytes.NewReader(stdin)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if c.Stderr != nil {
			_, _ = io.Copy(c.Stderr, &errBuf)
		}
		return nil, fmt.Errorf("kubectl exec -i %s/%s: %w", namespace, pod, err)
	}
	return out.Bytes(), nil
}

// DeletePods deletes pods in namespace selected by the given label selectors.
// Each selector is passed through as a -l argument.
func (c *CmdRunner) DeletePods(ctx context.Context, namespace string, selectors ...string) error {
	args := append(c.globalArgs(), "delete", "pod", "-n", namespace)
	for _, s := range selectors {
		args = append(args, "-l", s)
	}
	cmd := exec.CommandContext(ctx, c.kubectl(), args...)
	cmd.Stdout = c.Stderr
	cmd.Stderr = c.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl delete pod in %s: %w", namespace, err)
	}
	return nil
}

// FindBattleGroupNamespace returns the first namespace whose name has the
// funcom-seabass- prefix. Errors when none exists.
func FindBattleGroupNamespace(ctx context.Context, r Getter) (string, error) {
	out, err := r.Get(ctx, "ns", "--no-headers", "-o", "custom-columns=NAME:.metadata.name")
	if err != nil {
		return "", err
	}
	for _, name := range strings.Fields(string(out)) {
		if strings.HasPrefix(name, "funcom-seabass-") {
			return name, nil
		}
	}
	return "", fmt.Errorf("no funcom-seabass-* namespace found")
}

// BattleGroupName derives the BattleGroup name from its namespace.
func BattleGroupName(namespace string) string {
	return strings.TrimPrefix(namespace, "funcom-seabass-")
}

// NodeExternalIP returns the first node's ExternalIP as declared by the
// Kubernetes API. On a single-node K3s cluster behind DNAT this is the
// value written by etc/k3s/install.sh from K3S_NODE_EXTERNAL_IP, which
// is more reliable than an egress-based public-IP lookup when the VM
// shares an outbound NAT pool with other hosts.
func NodeExternalIP(ctx context.Context, r Runner) (string, error) {
	out, err := r.Get(ctx, "nodes",
		"-o", `jsonpath={.items[0].status.addresses[?(@.type=="ExternalIP")].address}`)
	if err != nil {
		return "", err
	}
	ip := strings.TrimSpace(string(out))
	if ip == "" {
		return "", fmt.Errorf("no ExternalIP set on first node")
	}
	return ip, nil
}
