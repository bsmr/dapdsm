package hostprep

import (
	"context"
	"fmt"
	"strings"
)

// ControlPlaneTaint checks whether control-plane nodes are tainted to repel
// general workloads. RKE2 and k3s leave control-plane nodes schedulable by
// default (unlike kubeadm), so in a multi-node cluster Funcom operators can
// inadvertently land on control-plane nodes.
//
// The check is advisory only: it detects the situation and reports an
// actionable hint. It never applies taints.
func ControlPlaneTaint(ctx context.Context, r Runner) Check {
	const name = "control-plane workload isolation"

	// Query 1: control-plane nodes and their taint effects.
	// Output format per node: "<name>=<effect1> <effect2> ;" (effects space-separated, may be empty).
	cpRes, err := r.Kubectl(ctx,
		"get", "nodes",
		"-l", "node-role.kubernetes.io/control-plane",
		"-o", `jsonpath={range .items[*]}{.metadata.name}{"="}{range .spec.taints[*]}{.effect}{" "}{end}{";"}{end}`,
	)
	if err != nil {
		return Check{
			Name:   name,
			OK:     false,
			Detail: fmt.Sprintf("could not query control-plane node taints: %v", err),
		}
	}

	// Parse "name=effects;" entries.
	type cpNode struct {
		name    string
		effects string
	}
	var cpNodes []cpNode
	for _, entry := range strings.Split(strings.TrimSpace(cpRes.Stdout), ";") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		idx := strings.Index(entry, "=")
		if idx < 0 {
			continue
		}
		cpNodes = append(cpNodes, cpNode{
			name:    entry[:idx],
			effects: entry[idx+1:],
		})
	}

	if len(cpNodes) == 0 {
		return Check{
			Name:   name,
			OK:     true,
			Detail: "no control-plane-labeled nodes (nothing to check)",
		}
	}

	// Query 2: worker nodes (nodes WITHOUT the control-plane label).
	workerRes, workerErr := r.Kubectl(ctx,
		"get", "nodes",
		"-l", "!node-role.kubernetes.io/control-plane",
		"-o", `jsonpath={.items[*].metadata.name}`,
	)
	if workerErr != nil {
		return Check{
			Name:   name,
			OK:     false,
			Detail: fmt.Sprintf("could not query worker nodes: %v", workerErr),
		}
	}
	workers := strings.TrimSpace(workerRes.Stdout)

	if workers == "" {
		return Check{
			Name:   name,
			OK:     true,
			Detail: "single-node cluster (no dedicated workers); control-plane stays schedulable by design",
		}
	}

	// Multi-node: verify every CP node has a repelling taint.
	// Only NoSchedule and NoExecute repel workloads; PreferNoSchedule does not.
	var schedulable []string
	for _, n := range cpNodes {
		tainted := false
		for _, tok := range strings.Fields(n.effects) {
			if tok == "NoSchedule" || tok == "NoExecute" {
				tainted = true
				break
			}
		}
		if !tainted {
			schedulable = append(schedulable, n.name)
		}
	}

	if len(schedulable) == 0 {
		return Check{
			Name:   name,
			OK:     true,
			Detail: fmt.Sprintf("all %d control-plane node(s) tainted (workloads kept off)", len(cpNodes)),
		}
	}

	return Check{
		Name: name,
		OK:   false,
		Detail: fmt.Sprintf(
			"control-plane nodes schedulable: %s — workloads (incl. Funcom operators) may land on them."+
				" RKE2/k3s leave control-plane schedulable by default; taint with CriticalAddonsOnly=true:NoExecute"+
				" (node-taint in the server config.yaml, applied at node registration)."+
				" doctor only reports this; it does not apply the taint.",
			strings.Join(schedulable, ", "),
		),
	}
}
