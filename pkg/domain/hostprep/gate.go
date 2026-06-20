// Package hostprep diagnoses and prepares a jumphost control host for Dune
// operations: it gates on cluster-admin access, checks the dune operations user,
// and remediates. It performs no cluster provisioning. See the design spec.
package hostprep

import (
	"context"
	"strings"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// Runner is what hostprep needs from the jumphost: kubectl against the cluster
// (Phase A) and arbitrary jumphost-local commands (Phase B / remediation).
// *clusteraccess.Access satisfies it.
type Runner interface {
	Kubectl(ctx context.Context, args ...string) (ssh.Result, error)
	OnJump(ctx context.Context, name string, args ...string) (ssh.Result, error)
}

// Probe is one cluster-access check.
type Probe struct {
	Name   string
	OK     bool
	Detail string
}

// Gate is the Phase-A cluster-access result. Pass is true only if every probe OK.
type Gate struct {
	Probes []Probe
	Pass   bool
}

// ClusterGate runs the Phase-A cluster-access RBAC gate via kubectl on the
// jumphost. Connectivity probes (get ns / get nodes) pass on a zero exit; the
// auth-can-i probes pass only when stdout is exactly "yes" (a "no" exits
// non-zero, so the error/exit must NOT be the signal). The stderr "not namespace
// scoped" warning that can-i prints for cluster-scoped resources is ignored.
func ClusterGate(ctx context.Context, r Runner) Gate {
	g := Gate{Pass: true}

	connectivity := []struct {
		name string
		args []string
	}{
		{"get namespaces", []string{"get", "ns"}},
		{"get nodes", []string{"get", "nodes", "-o", "wide"}},
	}
	for _, c := range connectivity {
		res, err := r.Kubectl(ctx, c.args...)
		p := Probe{Name: c.name, OK: err == nil}
		if !p.OK {
			p.Detail = strings.TrimSpace(res.Stderr)
		}
		g.Probes = append(g.Probes, p)
		g.Pass = g.Pass && p.OK
	}

	canI := []struct {
		name string
		res  string
	}{
		{"create namespaces", "namespace"},
		{"create CRDs", "customresourcedefinitions.apiextensions.k8s.io"},
		{"create clusterrolebindings", "clusterrolebindings.rbac.authorization.k8s.io"},
	}
	for _, c := range canI {
		res, _ := r.Kubectl(ctx, "auth", "can-i", "create", c.res)
		ok := strings.TrimSpace(res.Stdout) == "yes"
		p := Probe{Name: "can-i " + c.name, OK: ok}
		if !ok {
			p.Detail = "cannot create " + c.res + ": need a cluster-admin kubeconfig"
		}
		g.Probes = append(g.Probes, p)
		g.Pass = g.Pass && ok
	}
	return g
}
