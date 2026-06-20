// Package clusteraccess drives an already-provisioned, jumphost-fronted
// Kubernetes cluster: it parses a node inventory, runs kubectl on the
// jumphost, and runs commands on individual nodes. It performs no
// provisioning — see docs/dapdsm/specs/2026-06-20-ds-arrakis-cluster-bringup-design.md.
package clusteraccess

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// Role is a cluster node's role group from the inventory.
type Role string

const (
	RoleControlPlane Role = "controlplane"
	RoleWorker       Role = "worker"
	RoleLB           Role = "lb"
)

// Node is one cluster machine.
type Node struct {
	Name    string
	Address string
	Role    Role
}

// Descriptor is a resolved cluster: how to reach it and what it contains.
// All paths (Kubeconfig, NodeKey) are locations ON the jumphost; ds-arrakis
// never reads their contents.
type Descriptor struct {
	JumpHost   string // ssh-config alias of the jumphost (the control host)
	Kubeconfig string // kubeconfig path on the jumphost
	Distro     string // e.g. "rke2"
	NodeUser   string // ssh user for node access (from inventory vars)
	NodeKey    string // node ssh key path on the jumphost (from inventory vars)
	Nodes      []Node
}

// ansibleInventory is the subset of a standard Ansible inventory we consume.
type ansibleInventory struct {
	All struct {
		Children map[string]struct {
			Hosts map[string]struct {
				AnsibleHost string `yaml:"ansible_host"`
			} `yaml:"hosts"`
		} `yaml:"children"`
		Vars struct {
			User string `yaml:"ansible_user"`
			Key  string `yaml:"ansible_ssh_private_key_file"`
		} `yaml:"vars"`
	} `yaml:"all"`
}

// parseInventory parses a standard Ansible inventory into nodes plus the
// shared ssh user and key path. Groups other than controlplane/worker/lb are
// ignored. Nodes are sorted by name for deterministic output.
func parseInventory(data []byte) (nodes []Node, user, key string, err error) {
	var inv ansibleInventory
	if err = yaml.Unmarshal(data, &inv); err != nil {
		return nil, "", "", fmt.Errorf("parse inventory: %w", err)
	}
	roles := map[string]Role{
		"controlplane": RoleControlPlane,
		"worker":       RoleWorker,
		"lb":           RoleLB,
	}
	for group, body := range inv.All.Children {
		role, ok := roles[group]
		if !ok {
			continue
		}
		for name, h := range body.Hosts {
			nodes = append(nodes, Node{Name: name, Address: h.AnsibleHost, Role: role})
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })
	return nodes, inv.All.Vars.User, inv.All.Vars.Key, nil
}
