package operatorbringup

import (
	"context"
	"fmt"
	"strings"
)

// ops is the four Funcom operators, in apply/wait order.
var ops = []string{"battlegroupoperator", "databaseoperator", "serveroperator", "utilitiesoperator"}

const namespace = "funcom-operators"

// Kubectl is the cluster-access seam: Run for read/label/wait/apply-by-path,
// Apply to pipe a rendered manifest via `kubectl apply -f -`.
type Kubectl interface {
	Run(ctx context.Context, args ...string) (string, error)
	Apply(ctx context.Context, manifest []byte) (string, error)
}

// Options configure the operator bring-up.
type Options struct {
	Version        string // operator version (from version.txt)
	CRDDir         string // operator CRD dir on the jumphost
	CertManagerURL string // empty => skip cert-manager (assume present)
}

// BringUp performs the operator bring-up: cert-manager (optional) -> namespace ->
// worker discovery + labels -> CRDs -> webhook secrets -> operator Deployments+RBAC -> wait.
func BringUp(ctx context.Context, kc Kubectl, opts Options) error {
	if err := installCertManager(ctx, kc, opts.CertManagerURL); err != nil {
		return err
	}
	nsManifest := "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: " + namespace + "\n"
	if _, err := kc.Apply(ctx, []byte(nsManifest)); err != nil {
		return fmt.Errorf("apply namespace: %w", err)
	}
	workers, err := workerNodes(ctx, kc)
	if err != nil {
		return err
	}
	if err := labelWorkers(ctx, kc, workers); err != nil {
		return err
	}
	if _, err := kc.Run(ctx, "apply", "--server-side", "--validate=false", "-f", opts.CRDDir); err != nil {
		return fmt.Errorf("apply operator CRDs: %w", err)
	}
	for _, op := range ops {
		secret := op + "-webhook-server-cert"
		if _, err := kc.Run(ctx, "get", "secret", secret, "-n", namespace); err != nil {
			m, gerr := webhookSecret(op)
			if gerr != nil {
				return gerr
			}
			if _, aerr := kc.Apply(ctx, m); aerr != nil {
				return fmt.Errorf("apply webhook secret %s: %w", secret, aerr)
			}
		}
	}
	manifest, err := renderOperators(opts.Version)
	if err != nil {
		return err
	}
	if _, err := kc.Apply(ctx, manifest); err != nil {
		return fmt.Errorf("apply operator deployments: %w", err)
	}
	for _, op := range ops {
		if err := waitAvailable(ctx, kc, op+"-controller-manager"); err != nil {
			return err
		}
	}
	return nil
}

// workerNodes lists the cluster's worker nodes via the API: every node WITHOUT
// the control-plane role label. K8s-native + always current (new/removed workers
// are reflected automatically) — no inventory needed.
// ponytail: keys off node-role.kubernetes.io/control-plane (set by rke2 servers
// and k3s masters). Ceiling: a dedicated etcd-only node (labeled etcd but not
// control-plane) would be miscounted as a worker; add !…/etcd,!…/master to the
// selector if a cluster ever has standalone etcd nodes.
func workerNodes(ctx context.Context, kc Kubectl) ([]string, error) {
	out, err := kc.Run(ctx, "get", "nodes",
		"-l", "!node-role.kubernetes.io/control-plane",
		"-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return nil, fmt.Errorf("list worker nodes: %w", err)
	}
	workers := strings.Fields(out)
	if len(workers) == 0 {
		return nil, fmt.Errorf("no worker nodes found (all nodes control-plane?)")
	}
	return workers, nil
}

// installCertManager applies a caller-supplied cert-manager release idempotently.
// An empty url means the caller manages cert-manager out-of-band: skip entirely.
func installCertManager(ctx context.Context, kc Kubectl, url string) error {
	if url == "" {
		return nil
	}
	if _, err := kc.Run(ctx, "get", "deployment", "cert-manager", "-n", "cert-manager"); err == nil {
		return nil // already installed
	}
	if _, err := kc.Run(ctx, "apply", "--validate=false", "-f", url); err != nil {
		return fmt.Errorf("apply cert-manager %s: %w", url, err)
	}
	if _, err := kc.Run(ctx, "wait", "--for=condition=Available", "-n", "cert-manager",
		"deployment/cert-manager", "deployment/cert-manager-webhook", "deployment/cert-manager-cainjector",
		"--timeout=300s"); err != nil {
		return fmt.Errorf("wait cert-manager available: %w", err)
	}
	return nil
}

// labelWorkers labels every worker node as Funcom infrastructure.
func labelWorkers(ctx context.Context, kc Kubectl, workers []string) error {
	for _, w := range workers {
		if _, err := kc.Run(ctx, "label", "node", w, "node.funcom.com/workload=infrastructure", "--overwrite"); err != nil {
			return fmt.Errorf("label node %s: %w", w, err)
		}
	}
	return nil
}

// waitAvailable blocks until the named deployment reports Available.
func waitAvailable(ctx context.Context, kc Kubectl, deployment string) error {
	if _, err := kc.Run(ctx, "wait", "--for=condition=Available", "-n", namespace,
		"deployment/"+deployment, "--timeout=180s"); err != nil {
		return fmt.Errorf("wait %s available: %w", deployment, err)
	}
	return nil
}
