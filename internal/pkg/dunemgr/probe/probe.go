// Package probe runs a one-shot status check against a host.
package probe

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/tunnel"
	"go.muehmer.eu/dapdsm/internal/pkg/kube"
)

// newKubeRunner builds the Runner used to talk to the tunneled API
// server. Overridable in tests.
var newKubeRunner = func(kubeconfigPath string, stderr io.Writer) kube.Runner {
	return &kube.CmdRunner{Kubeconfig: kubeconfigPath, Stderr: stderr}
}

// Probe opens the K8s API tunnel (idempotent), writes a throwaway
// kubeconfig, reads the BattleGroup CR's observed status, writes the
// snapshot to the store, and returns it. Transport/parse failures are
// recorded on the snapshot (BGState UNKNOWN + Error) rather than
// returned as a hard error, so the poller keeps running.
func Probe(ctx context.Context, s *store.Store, tm *tunnel.Manager, host string) (store.StatusSnapshot, error) {
	prof, err := s.GetHost(host)
	if err != nil {
		return store.StatusSnapshot{}, err
	}
	now := time.Now().UTC()

	fail := func(msg string) (store.StatusSnapshot, error) {
		snap := store.StatusSnapshot{Host: host, ProbedAt: now, BGState: "UNKNOWN", Error: msg}
		_ = s.PutStatus(snap)
		return snap, nil
	}

	port, err := tm.OpenSlot(ctx, host, "127.0.0.1", 6443)
	if err != nil {
		return fail(err.Error())
	}
	kubeconfigPath := filepath.Join(tunnel.RuntimeDir(), "kubeconfig-"+host)
	if err := kube.WriteKubeconfig(kubeconfigPath, port, prof.FQDN, prof.K3sCABase64); err != nil {
		return fail(err.Error())
	}

	r := newKubeRunner(kubeconfigPath, os.Stderr)
	st, err := probeStatus(ctx, r)
	if err != nil {
		return fail(err.Error())
	}

	snap := snapshotFromStatus(host, st, now)
	if err := s.PutStatus(snap); err != nil {
		return snap, err
	}
	return snap, nil
}
