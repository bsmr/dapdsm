package store

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
)

// AuditEntry records one operator action. Keys are 8-byte big-endian
// sequence numbers so List iterates chronologically.
type AuditEntry struct {
	TS       time.Time `json:"ts"`
	Operator string    `json:"operator"`
	Host     string    `json:"host,omitempty"`
	Action   string    `json:"action"`
	Subject  string    `json:"subject,omitempty"`
	Result   string    `json:"result,omitempty"`
	Diff     string    `json:"diff,omitempty"`
}

// AppendAudit writes a new entry. Sequence number is allocated by
// bbolt; entries are immutable after write.
func (s *Store) AppendAudit(e AuditEntry) error {
	if e.TS.IsZero() {
		e.TS = time.Now().UTC()
	}
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("audit"))
		seq, err := b.NextSequence()
		if err != nil {
			return err
		}
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, seq)
		return b.Put(key, data)
	})
}

// ListAudit returns up to limit entries, oldest first. limit<=0
// returns all entries.
func (s *Store) ListAudit(limit int) ([]AuditEntry, error) {
	var out []AuditEntry
	err := s.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket([]byte("audit")).Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if limit > 0 && len(out) >= limit {
				break
			}
			var e AuditEntry
			if err := json.Unmarshal(v, &e); err != nil {
				return fmt.Errorf("unmarshal: %w", err)
			}
			out = append(out, e)
		}
		return nil
	})
	return out, err
}
