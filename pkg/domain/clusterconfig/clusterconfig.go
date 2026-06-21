// Package clusterconfig stores a dapdsm-owned configuration layer in the
// cluster: a ConfigMap (non-secret values) plus a Secret (secret values) in a
// dapdsm namespace. It is the source of truth once the cluster is up, and is
// reusable by any subsystem (ds-bashar bring-up today; bots/ds-spice later).
package clusterconfig

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrNotFound means the named ConfigMap does not exist yet (fresh cluster).
var ErrNotFound = errors.New("clusterconfig: not found")

// Kubectl is the minimal cluster seam: read with Get, write with Apply (-f -).
type Kubectl interface {
	Get(ctx context.Context, args ...string) ([]byte, error)
	Apply(ctx context.Context, manifest []byte) error
}

// Data is one config record: plain Values (ConfigMap) + Secrets (Secret).
type Data struct {
	Values  map[string]string
	Secrets map[string][]byte
}

// Store reads/writes records in a fixed namespace.
type Store struct {
	KC        Kubectl
	Namespace string
}

// Save renders the ConfigMap + Secret and applies both idempotently.
func (s Store) Save(ctx context.Context, name string, d Data) error {
	cm := renderConfigMap(s.Namespace, name, d.Values)
	if err := s.KC.Apply(ctx, []byte(cm)); err != nil {
		return fmt.Errorf("apply configmap %s: %w", name, err)
	}
	sec := renderSecret(s.Namespace, name+"-secrets", d.Secrets)
	if err := s.KC.Apply(ctx, []byte(sec)); err != nil {
		return fmt.Errorf("apply secret %s-secrets: %w", name, err)
	}
	return nil
}

// Load reads the merged ConfigMap+Secret JSON and decodes it. The Get call
// fetches both objects' data maps as one JSON document (see args below).
func (s Store) Load(ctx context.Context, name string) (Data, error) {
	// jq-free: ask kubectl for a tiny composite via two -o jsonpath reads would
	// need two calls; instead read both as json and merge here.
	out, err := s.KC.Get(ctx, "get",
		"configmap", name, "secret", name+"-secrets",
		"-n", s.Namespace, "-o",
		`jsonpath={"{\"cm\":"}{.items[0].data}{,\"secret\":}{.items[1].data}{"}"}`)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "not found") {
			return Data{}, ErrNotFound
		}
		return Data{}, fmt.Errorf("get %s: %w", name, err)
	}
	var raw struct {
		CM     map[string]string `json:"cm"`
		Secret map[string]string `json:"secret"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return Data{}, fmt.Errorf("parse %s: %w", name, err)
	}
	d := Data{Values: raw.CM, Secrets: map[string][]byte{}}
	for k, v := range raw.Secret {
		dec, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return Data{}, fmt.Errorf("decode secret %s: %w", k, err)
		}
		d.Secrets[k] = dec
	}
	return d, nil
}

func renderConfigMap(ns, name string, vals map[string]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: %s\n  namespace: %s\ndata:\n", name, ns)
	for _, k := range sortedKeys(vals) {
		fmt.Fprintf(&b, "  %s: %s\n", k, vals[k])
	}
	return b.String()
}

func renderSecret(ns, name string, secrets map[string][]byte) string {
	var b strings.Builder
	fmt.Fprintf(&b, "apiVersion: v1\nkind: Secret\nmetadata:\n  name: %s\n  namespace: %s\ntype: Opaque\nstringData:\n", name, ns)
	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "  %s: %s\n", k, string(secrets[k]))
	}
	return b.String()
}

func sortedKeys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
