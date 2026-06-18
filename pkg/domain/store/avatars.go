package store

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.etcd.io/bbolt"
)

// ExportRecord points at one per-avatar character-transfer export dumped to
// the workstation (the JSON returned by dune.character_transfer_export).
type ExportRecord struct {
	Host          string    `json:"host"`
	FLSID         string    `json:"fls_id"`
	CharacterName string    `json:"character_name"`
	UnixTS        int64     `json:"unix_ts"`
	LocalPath     string    `json:"local_path"`
	Bytes         int64     `json:"bytes"`
	Checksum      string    `json:"checksum"` // _patches_checksum embedded in the dump
	Operator      string    `json:"operator"`
	CreatedAt     time.Time `json:"created_at"`
}

// Key returns the bbolt key for this record. "/" delimits host from the
// fls-id/timestamp segment, keeping the key printable and copy-pasteable
// (e.g. "vm-a/DEADBEEF-42"). The prefix scan in ListExports stays unambiguous
// because host aliases contain no "/".
func (e ExportRecord) Key() string {
	return fmt.Sprintf("%s/%s-%d", e.Host, e.FLSID, e.UnixTS)
}

// PutExport inserts or replaces a record.
func (s *Store) PutExport(rec ExportRecord) error {
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now().UTC()
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte("avatars")).Put([]byte(rec.Key()), data)
	})
}

// GetExport loads one record by key. Returns ErrNotFound if unknown.
func (s *Store) GetExport(key string) (ExportRecord, error) {
	var rec ExportRecord
	err := s.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket([]byte("avatars")).Get([]byte(key))
		if v == nil {
			return ErrNotFound
		}
		return json.Unmarshal(v, &rec)
	})
	return rec, err
}

// ListExports returns all records for one host, newest first.
func (s *Store) ListExports(host string) ([]ExportRecord, error) {
	prefix := []byte(host + "/")
	var out []ExportRecord
	err := s.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket([]byte("avatars")).Cursor()
		for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), string(prefix)); k, v = c.Next() {
			var rec ExportRecord
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
