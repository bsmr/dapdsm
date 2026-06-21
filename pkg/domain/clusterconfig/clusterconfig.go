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

// Save ensures the namespace exists, then renders the ConfigMap + Secret and
// applies all three idempotently (batteries-included: a fresh cluster has no
// dapdsm namespace yet).
func (s Store) Save(ctx context.Context, name string, d Data) error {
	ns := "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: " + s.Namespace + "\n"
	if err := s.KC.Apply(ctx, []byte(ns)); err != nil {
		return fmt.Errorf("apply namespace %s: %w", s.Namespace, err)
	}
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

// Load reads the ConfigMap and Secret separately with `kubectl get -o json` and
// merges their `.data` maps. The ConfigMap is the existence signal: if it is
// absent the record does not exist yet (ErrNotFound). A missing Secret is
// tolerated (no secrets stored yet). Two plain `-o json` reads avoid the
// fragile single-call jsonpath composite that real kubectl rejects.
func (s Store) Load(ctx context.Context, name string) (Data, error) {
	cmRaw, err := s.KC.Get(ctx, "get", "configmap", name, "-n", s.Namespace, "-o", "json")
	if err != nil {
		if isNotFound(err) {
			return Data{}, ErrNotFound
		}
		return Data{}, fmt.Errorf("get configmap %s: %w", name, err)
	}
	values, err := dataMap(cmRaw)
	if err != nil {
		return Data{}, fmt.Errorf("parse configmap %s: %w", name, err)
	}

	secRaw, err := s.KC.Get(ctx, "get", "secret", name+"-secrets", "-n", s.Namespace, "-o", "json")
	if err != nil {
		if isNotFound(err) {
			// ConfigMap exists but no Secret yet: a valid record with no secrets.
			return Data{Values: values, Secrets: map[string][]byte{}}, nil
		}
		return Data{}, fmt.Errorf("get secret %s-secrets: %w", name, err)
	}
	encoded, err := dataMap(secRaw)
	if err != nil {
		return Data{}, fmt.Errorf("parse secret %s-secrets: %w", name, err)
	}
	secrets := make(map[string][]byte, len(encoded))
	for k, v := range encoded {
		dec, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return Data{}, fmt.Errorf("decode secret %s: %w", k, err)
		}
		secrets[k] = dec
	}
	return Data{Values: values, Secrets: secrets}, nil
}

// isNotFound reports whether err is kubectl's "resource not found" (the message
// is surfaced from the command's stderr by the clusteraccess adapter).
func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "not found")
}

// dataMap extracts the `.data` object from a `kubectl get -o json` document.
// A resource with no data field yields an empty map, not an error.
func dataMap(raw []byte) (map[string]string, error) {
	var obj struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	if obj.Data == nil {
		obj.Data = map[string]string{}
	}
	return obj.Data, nil
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
