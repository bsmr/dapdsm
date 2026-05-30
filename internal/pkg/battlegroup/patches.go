// Package battlegroup generates RFC 6902 JSON-Patch operations against a
// Funcom BattleGroup Custom Resource (CR).
//
// The CR is consumed as raw JSON because the upstream Funcom CRDs are
// not vendored in this repository; treating it as a generic JSON tree
// keeps this package decoupled from Funcom's schema. Two transformations
// are produced:
//
//  1. HOST_DATACENTER_IP_ADDRESS env-var replacements: every occurrence
//     anywhere in the tree gets its value replaced with the operator-
//     supplied IP. The Funcom world template hard-codes 127.0.0.1 in
//     several places (director / serverGateway / textRouter), so a
//     generic walk is safer than addressing fixed paths whose indices
//     drift between Funcom releases.
//
//  2. schedulerName removals: the world template references a custom
//     "memory-focused-scheduler" that only exists inside the Funcom
//     Hyper-V appliance. On a fresh k3s host that scheduler is absent,
//     so every set carrying it must drop the field to fall back to the
//     default Kubernetes scheduler.
//
// Both transformations are pure: they read a CR and return operations.
// Applying them to the live cluster is the caller's job.
package battlegroup

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// HostDatacenterIPEnvVar is the env-var name whose value advertises the
// public IP through which Funcom Live Services reaches the dedicated
// server during the matchmaking handshake.
const HostDatacenterIPEnvVar = "HOST_DATACENTER_IP_ADDRESS"

// HostDatacenterIDEnvVar is the env-var name carrying a short
// human-readable host identifier (Funcom's vendor template ships this
// hard-coded as "dune-testing"). The Funcom-Operator writes it together
// with HOST_DATACENTER_IP_ADDRESS into the /etc/hosts of the Director /
// Server-Gateway / Text-Router pods, so the server-browser-ping check
// resolves "<id> → <ip>".
const HostDatacenterIDEnvVar = "HOST_DATACENTER_ID"

// CustomScheduler is the Funcom-Hyper-V-only scheduler that must be
// stripped from each ServerSet on a fresh k3s host.
const CustomScheduler = "memory-focused-scheduler"

// Operation is a single RFC 6902 JSON-Patch operation. Value is left as
// any so it can be omitted for "remove" operations.
type Operation struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

// BuildHostIPPatches returns one replace-operation per HOST_DATACENTER_IP_ADDRESS
// occurrence in cr whose current value differs from newIP. Occurrences that
// already carry newIP are skipped so the resulting patch is idempotent.
func BuildHostIPPatches(cr []byte, newIP string) ([]Operation, error) {
	return buildEnvVarPatches(cr, HostDatacenterIPEnvVar, newIP)
}

// BuildHostIDPatches returns one replace-operation per HOST_DATACENTER_ID
// occurrence in cr whose current value differs from newID. The Funcom
// template ships this hard-coded as "dune-testing"; setting it to a
// host-specific identifier makes the /etc/hosts entry on Director /
// Server-Gateway / Text-Router pods read "<id> → <public-ip>" which is
// more informative for diagnostics. Passing an empty newID returns nil
// so the caller can opt out without branching at the call-site.
func BuildHostIDPatches(cr []byte, newID string) ([]Operation, error) {
	if newID == "" {
		return nil, nil
	}
	return buildEnvVarPatches(cr, HostDatacenterIDEnvVar, newID)
}

// buildEnvVarPatches walks the CR for every envVars entry named name and
// emits add/replace-ops to bring each occurrence to newValue. Idempotent.
func buildEnvVarPatches(cr []byte, name, newValue string) ([]Operation, error) {
	root, err := decode(cr)
	if err != nil {
		return nil, err
	}
	var ops []Operation
	walk(root, nil, func(path []string, node any) {
		envVars, ok := envVarsAt(node)
		if !ok {
			return
		}
		for i, ev := range envVars {
			entry, ok := ev.(map[string]any)
			if !ok {
				continue
			}
			if n, _ := entry["name"].(string); n != name {
				continue
			}
			if cur, _ := entry["value"].(string); cur == newValue {
				continue
			}
			opType := "replace"
			if _, present := entry["value"]; !present {
				opType = "add"
			}
			ops = append(ops, Operation{
				Op:    opType,
				Path:  pointer(append(slices.Clone(path), "envVars", strconv.Itoa(i), "value")),
				Value: newValue,
			})
		}
	})
	return ops, nil
}

// BuildSchedulerNameRemovals returns one remove-operation per entry in
// spec.serverGroup.template.spec.sets whose schedulerName equals
// CustomScheduler. The indices are reported in descending order so that
// applying them sequentially does not shift the remaining targets.
func BuildSchedulerNameRemovals(cr []byte) ([]Operation, error) {
	root, err := decode(cr)
	if err != nil {
		return nil, err
	}
	sets, ok := dig(root, "spec", "serverGroup", "template", "spec", "sets").([]any)
	if !ok {
		return nil, nil
	}
	var indices []int
	for i, item := range sets {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if name, _ := entry["schedulerName"].(string); name == CustomScheduler {
			indices = append(indices, i)
		}
	}
	slices.Sort(indices)
	slices.Reverse(indices)
	ops := make([]Operation, 0, len(indices))
	for _, i := range indices {
		ops = append(ops, Operation{
			Op:   "remove",
			Path: fmt.Sprintf("/spec/serverGroup/template/spec/sets/%d/schedulerName", i),
		})
	}
	return ops, nil
}

func decode(cr []byte) (any, error) {
	var root any
	if err := json.Unmarshal(cr, &root); err != nil {
		return nil, fmt.Errorf("decode BattleGroup JSON: %w", err)
	}
	return root, nil
}

// walk performs a deterministic, depth-first traversal of a decoded JSON
// tree. Map entries are visited in sorted key order so generated patches
// are reproducible across runs.
func walk(node any, path []string, fn func(path []string, node any)) {
	fn(path, node)
	switch n := node.(type) {
	case map[string]any:
		keys := make([]string, 0, len(n))
		for k := range n {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			walk(n[k], append(slices.Clone(path), k), fn)
		}
	case []any:
		for i, v := range n {
			walk(v, append(slices.Clone(path), strconv.Itoa(i)), fn)
		}
	}
}

func envVarsAt(node any) ([]any, bool) {
	obj, ok := node.(map[string]any)
	if !ok {
		return nil, false
	}
	vars, ok := obj["envVars"].([]any)
	return vars, ok
}

func dig(root any, keys ...string) any {
	cur := root
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[k]
	}
	return cur
}

// pointer renders a path slice as an RFC 6901 JSON-Pointer string.
func pointer(parts []string) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteByte('/')
		b.WriteString(escapePointerToken(p))
	}
	return b.String()
}

func escapePointerToken(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}
