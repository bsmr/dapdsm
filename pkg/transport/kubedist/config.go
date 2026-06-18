package kubedist

import "context"

// Config holds the node addressing parameters used to render K8s
// distribution drop-in configuration files.
type Config struct {
	ExternalIP string
	InternalIP string
	TLSSANs    []string
}

// Distro abstracts the lifecycle of a Kubernetes distribution (k3s, rke2)
// on a single node.
type Distro interface {
	Name() string
	Install(ctx context.Context, cfg Config) error
	EnsureReady(ctx context.Context) error
	ImportImages(ctx context.Context, tarDir string) error
	Kubeconfig() string
	Uninstall(ctx context.Context) error
}
