// Package probe runs a one-shot status check against a host by reading
// the BattleGroup CR's observed status via SSH→node-kubectl. The node's
// own kubeconfig authenticates; dunemgr needs no tunnel or stored CA.
package probe

import (
	"context"
	"time"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/kube"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// ProbeTimeout bounds a single probe so a slow or hung kubectl can never
// wedge the poller.
const ProbeTimeout = 15 * time.Second

// sshGetter runs `kubectl get …` on the node over SSH.
type sshGetter struct {
	ssh  *ssh.Client
	host string
}

func (g sshGetter) Get(ctx context.Context, args ...string) ([]byte, error) {
	res, err := g.ssh.Run(ctx, g.host, "kubectl", append([]string{"get"}, args...)...)
	if err != nil {
		return nil, err
	}
	return []byte(res.Stdout), nil
}

// newGetter builds the kube.Getter used to read the cluster. Overridable
// in tests.
var newGetter = func(sshc *ssh.Client, host string) kube.Getter {
	return sshGetter{ssh: sshc, host: host}
}

// Probe reads the BattleGroup CR's observed status over SSH and writes a
// snapshot to the store. Transport/parse failures are recorded on the
// snapshot (BGState UNKNOWN + Error) rather than returned as a hard error,
// so the poller keeps running. A hard error is returned only when the host
// is not registered.
func Probe(ctx context.Context, s *store.Store, sshc *ssh.Client, host string) (store.StatusSnapshot, error) {
	if _, err := s.GetHost(host); err != nil {
		return store.StatusSnapshot{}, err
	}
	now := time.Now().UTC()

	pctx, cancel := context.WithTimeout(ctx, ProbeTimeout)
	defer cancel()

	st, err := probeStatus(pctx, newGetter(sshc, host))
	if err != nil {
		snap := store.StatusSnapshot{Host: host, ProbedAt: now, BGState: "UNKNOWN", Error: err.Error()}
		_ = s.PutStatus(snap)
		return snap, nil
	}

	snap := snapshotFromStatus(host, st, now)
	if err := s.PutStatus(snap); err != nil {
		return snap, err
	}
	return snap, nil
}
