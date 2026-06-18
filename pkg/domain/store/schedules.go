package store

import (
	"encoding/json"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
)

// ScheduledShutdown is one pending countdown-shutdown for a host.
// Keyed by host: one pending shutdown per host (re-scheduling replaces).
type ScheduledShutdown struct {
	Host               string    `json:"host"`
	Kind               string    `json:"kind"`   // Funcom ShutdownType: Restart|Maintenance|Update
	Action             string    `json:"action"` // lifecycle verb to run at the deadline: stop|restart
	NowUnix            int64     `json:"now_unix"`
	AtUnix             int64     `json:"at_unix"`
	ShutdownDurationS  int       `json:"shutdown_duration_s"`
	BroadcastFrequency int       `json:"broadcast_frequency"`
	BroadcastDuration  int       `json:"broadcast_duration"`
	Operator           string    `json:"operator"`
	CreatedAt          time.Time `json:"created_at"`
}

// PutSchedule inserts or replaces the pending shutdown for rec.Host.
func (s *Store) PutSchedule(rec ScheduledShutdown) error {
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now().UTC()
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte("schedules")).Put([]byte(rec.Host), data)
	})
}

// GetSchedule loads the pending shutdown for host, or ErrNotFound.
func (s *Store) GetSchedule(host string) (ScheduledShutdown, error) {
	var rec ScheduledShutdown
	err := s.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket([]byte("schedules")).Get([]byte(host))
		if v == nil {
			return ErrNotFound
		}
		return json.Unmarshal(v, &rec)
	})
	return rec, err
}

// DeleteSchedule removes the pending shutdown for host (no-op if absent).
func (s *Store) DeleteSchedule(host string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte("schedules")).Delete([]byte(host))
	})
}

// ListSchedules returns every pending shutdown across all hosts.
func (s *Store) ListSchedules() ([]ScheduledShutdown, error) {
	var out []ScheduledShutdown
	err := s.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte("schedules")).ForEach(func(_, v []byte) error {
			var rec ScheduledShutdown
			if err := json.Unmarshal(v, &rec); err != nil {
				return fmt.Errorf("unmarshal: %w", err)
			}
			out = append(out, rec)
			return nil
		})
	})
	return out, err
}
