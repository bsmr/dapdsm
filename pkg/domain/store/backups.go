package store

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.etcd.io/bbolt"
)

// BackupRecord points at one downloaded Funcom backup pair
// (.backup + .backup.yaml) on the workstation.
type BackupRecord struct {
	Host      string    `json:"host"`
	BG        string    `json:"bg"`
	Name      string    `json:"name"`
	UnixTS    int64     `json:"unix_ts"`
	LocalPath string    `json:"local_path"`
	Bytes     int64     `json:"bytes"`
	YAMLBytes int64     `json:"yaml_bytes"`
	Operator  string    `json:"operator"`
	CreatedAt time.Time `json:"created_at"`
}

// Key returns the bbolt key for this record. "/" delimits host, bg, and the
// timestamp+name segment, keeping the key printable and copy-pasteable
// (e.g. "vm-a/sietch/7__nightly"). The prefix scan in ListBackups stays
// unambiguous because host aliases and BG names contain no "/".
func (r BackupRecord) Key() string {
	return fmt.Sprintf("%s/%s/%d__%s", r.Host, r.BG, r.UnixTS, r.Name)
}

// PutBackup inserts or replaces a record.
func (s *Store) PutBackup(rec BackupRecord) error {
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now().UTC()
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte("backups")).Put([]byte(rec.Key()), data)
	})
}

// GetBackup loads a single record by key. Returns ErrNotFound if
// the key is unknown.
func (s *Store) GetBackup(key string) (BackupRecord, error) {
	var rec BackupRecord
	err := s.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket([]byte("backups")).Get([]byte(key))
		if v == nil {
			return ErrNotFound
		}
		return json.Unmarshal(v, &rec)
	})
	return rec, err
}

// DeleteBackup removes a record (no-op if not present).
func (s *Store) DeleteBackup(key string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte("backups")).Delete([]byte(key))
	})
}

// ListBackups returns all records for one (host, bg) pair, newest
// first.
func (s *Store) ListBackups(host, bg string) ([]BackupRecord, error) {
	prefix := []byte(host + "/" + bg + "/")
	var out []BackupRecord
	err := s.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket([]byte("backups")).Cursor()
		for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = c.Next() {
			var rec BackupRecord
			if err := json.Unmarshal(v, &rec); err != nil {
				return fmt.Errorf("unmarshal: %w", err)
			}
			out = append(out, rec)
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].UnixTS > out[j].UnixTS })
	return out, err
}
