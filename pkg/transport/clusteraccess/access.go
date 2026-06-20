package clusteraccess

import (
	"context"
	"fmt"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// Access drives a resolved cluster via its jumphost.
type Access struct {
	ex Execer
	d  *Descriptor
}

// New returns an Access bound to a descriptor and execer.
func New(ex Execer, d *Descriptor) *Access { return &Access{ex: ex, d: d} }

// Kubectl runs `kubectl <args...>` on the jumphost with KUBECONFIG exported to
// the descriptor's kubeconfig path. `env` sets the variable for that one
// process without relying on the remote login shell.
func (a *Access) Kubectl(ctx context.Context, args ...string) (ssh.Result, error) {
	full := append([]string{"KUBECONFIG=" + a.d.Kubeconfig, "kubectl"}, args...)
	return a.ex.Run(ctx, a.d.JumpHost, "env", full...)
}

// Nodes returns the descriptor's nodes filtered by role; role == "" returns all.
func (a *Access) Nodes(role Role) []Node {
	if role == "" {
		return a.d.Nodes
	}
	var out []Node
	for _, n := range a.d.Nodes {
		if n.Role == role {
			out = append(out, n)
		}
	}
	return out
}

// node looks up a node by name.
func (a *Access) node(name string) (Node, error) {
	for _, n := range a.d.Nodes {
		if n.Name == name {
			return n, nil
		}
	}
	return Node{}, fmt.Errorf("node %q not in cluster %s", name, a.d.JumpHost)
}

// OnJump runs an arbitrary command on the jumphost (the control host). Used by
// host-prep tooling that operates on the jumphost itself rather than the cluster.
func (a *Access) OnJump(ctx context.Context, name string, args ...string) (ssh.Result, error) {
	return a.ex.Run(ctx, a.d.JumpHost, name, args...)
}

// OnNode runs cmd on the named node through a jumphost->node ssh hop. The node
// key is referenced as an `ssh -i` argument executed by the jumphost's ssh
// binary — ds-arrakis never reads the key (CLAUDE.md access rule).
//
// ponytail: the outer jump hop IS quoted (ssh.Client.Run shell-quotes every
// token), but the jumphost's ssh then concatenates the post-target args with
// spaces and the node shell re-splits them — so multi-token cmd survives, yet a
// single cmd element containing spaces would split on the node. Keep each
// element space-free (true for S0 callers: `true`, ctr import with plain paths).
// Revisit if a node command needs an element with embedded spaces.
func (a *Access) OnNode(ctx context.Context, node string, cmd ...string) (ssh.Result, error) {
	if a.d.Distro != "rke2" {
		return ssh.Result{}, fmt.Errorf("OnNode: distro %q has no ssh node access (S0 implements rke2 only)", a.d.Distro)
	}
	n, err := a.node(node)
	if err != nil {
		return ssh.Result{}, err
	}
	target := a.d.NodeUser + "@" + n.Address
	args := append([]string{"-i", a.d.NodeKey, "-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no", target}, cmd...)
	return a.ex.Run(ctx, a.d.JumpHost, "ssh", args...)
}
