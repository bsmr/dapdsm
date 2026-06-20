package clusteraccess

import (
	"context"
	"fmt"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// Execer runs a command on a host (an ssh-config alias). *ssh.Client satisfies
// it; tests inject a fake. The host is always the jumphost in this package.
type Execer interface {
	Run(ctx context.Context, host, cmd string, args ...string) (ssh.Result, error)
}

// LoadParams locates a cluster's descriptor files on the jumphost.
type LoadParams struct {
	JumpHost       string // ssh-config alias of the jumphost
	KubeconfigPath string // kubeconfig path on the jumphost
	InventoryPath  string // Ansible inventory path on the jumphost
	Distro         string // optional; defaults to "rke2"
}

// Load reads the inventory off the jumphost and assembles a Descriptor. It
// never fetches kubeconfig, token, or key *contents* — only their paths.
func Load(ctx context.Context, ex Execer, p LoadParams) (*Descriptor, error) {
	res, err := ex.Run(ctx, p.JumpHost, "cat", p.InventoryPath)
	if err != nil {
		return nil, fmt.Errorf("read inventory %s: %w", p.InventoryPath, err)
	}
	nodes, user, key, err := parseInventory([]byte(res.Stdout))
	if err != nil {
		return nil, err
	}
	distro := p.Distro
	if distro == "" {
		distro = "rke2"
	}
	return &Descriptor{
		JumpHost:   p.JumpHost,
		Kubeconfig: p.KubeconfigPath,
		Distro:     distro,
		NodeUser:   user,
		NodeKey:    key,
		Nodes:      nodes,
	}, nil
}
