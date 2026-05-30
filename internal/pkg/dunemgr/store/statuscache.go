package store

import (
	"encoding/json"
	"fmt"
	"time"

	"go.etcd.io/bbolt"

	"go.muehmer.eu/dapdsm/internal/pkg/battlegroup"
)

// StatusSnapshot is the last-known status for one host. Refreshed
// by the host-probe loop; consumed by the UI and aggregate
// dashboard.
type StatusSnapshot struct {
	Host     string    `json:"host"`
	ProbedAt time.Time `json:"probed_at"`
	BGState  string    `json:"bg_state"`  // uppercased serverGroupPhase, or "UNKNOWN"
	PodReady int       `json:"pod_ready"` // ready servers (was: ready pods)
	PodTotal int       `json:"pod_total"` // total servers (was: total pods)
	// Detail is the full parsed BattleGroup status powering the
	// Lifecycle-tab header + per-map table. Zero value when unprobed
	// or on error.
	Detail battlegroup.Status `json:"detail"`
	Error  string             `json:"error,omitempty"`
}

// PutStatus replaces the cached snapshot for s.Host.
func (s *Store) PutStatus(snap StatusSnapshot) error {
	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte("statuscache")).Put([]byte(snap.Host), data)
	})
}

// GetStatus returns the cached snapshot for the given host, or
// ErrNotFound.
func (s *Store) GetStatus(host string) (StatusSnapshot, error) {
	var snap StatusSnapshot
	err := s.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket([]byte("statuscache")).Get([]byte(host))
		if v == nil {
			return ErrNotFound
		}
		return json.Unmarshal(v, &snap)
	})
	return snap, err
}
