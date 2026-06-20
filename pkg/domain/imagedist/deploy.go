package imagedist

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"text/template"
)

// Kubectl is the cluster-access seam Deploy needs: Run for read/wait verbs and
// Apply to pipe a rendered manifest via `kubectl apply -f -`.
type Kubectl interface {
	Run(ctx context.Context, args ...string) (string, error)
	Apply(ctx context.Context, manifest []byte) (string, error)
}

// Options configure the in-cluster registry. Endpoint is the host:port nodes use
// to pull (must match the provisioner-seeded registries.yaml). StorageClass and
// LoadBalancerIP are provisioner-coordinated prerequisites.
type Options struct {
	Namespace      string
	StorageClass   string
	RegistryImage  string
	Endpoint       string // host:port (used by Push; Port is derived for the Service)
	PVCSize        string
	LoadBalancerIP string
}

// Port returns the numeric port from Endpoint (host:port), defaulting to 5000.
func (o Options) Port() int {
	if i := strings.LastIndex(o.Endpoint, ":"); i >= 0 {
		if p, err := strconv.Atoi(o.Endpoint[i+1:]); err == nil {
			return p
		}
	}
	return 5000
}

// render produces the registry manifest from opts.
func render(opts Options) ([]byte, error) {
	t, err := template.New("registry").Parse(registryTmpl)
	if err != nil {
		return nil, fmt.Errorf("parse registry template: %w", err)
	}
	data := struct {
		Options
		Port int
	}{opts, opts.Port()}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render registry manifest: %w", err)
	}
	return buf.Bytes(), nil
}

// Deploy renders the registry manifest, applies it, and waits for the registry
// Deployment to roll out. Idempotent (apply is declarative).
func Deploy(ctx context.Context, kc Kubectl, opts Options) error {
	manifest, err := render(opts)
	if err != nil {
		return err
	}
	if _, err := kc.Apply(ctx, manifest); err != nil {
		return fmt.Errorf("apply registry manifest: %w", err)
	}
	if _, err := kc.Run(ctx, "-n", opts.Namespace, "rollout", "status",
		"deployment/registry", "--timeout=120s"); err != nil {
		return fmt.Errorf("wait for registry rollout: %w", err)
	}
	return nil
}
