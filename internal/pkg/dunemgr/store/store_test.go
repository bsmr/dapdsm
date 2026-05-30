package store

import (
	"path/filepath"
	"testing"
)

func TestOpenCreatesAllBuckets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.bolt")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	wantBuckets := []string{"hosts", "audit", "backups", "schedules", "statuscache"}
	for _, b := range wantBuckets {
		if !s.HasBucket(b) {
			t.Errorf("missing bucket %q", b)
		}
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.bolt")
	s1, err := Open(path)
	if err != nil {
		t.Fatalf("Open #1: %v", err)
	}
	s1.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("Open #2: %v", err)
	}
	defer s2.Close()
	if !s2.HasBucket("hosts") {
		t.Errorf("bucket missing on reopen")
	}
}
