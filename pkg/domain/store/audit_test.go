package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAppendAuditAndList(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "state.bolt"))
	defer s.Close()

	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	e1 := AuditEntry{TS: now, Operator: "local", Host: "vm-a", Action: "lifecycle.start"}
	e2 := AuditEntry{TS: now.Add(time.Minute), Operator: "local", Host: "vm-a", Action: "db.exec", Subject: "SELECT 1"}

	if err := s.AppendAudit(e1); err != nil {
		t.Fatalf("AppendAudit e1: %v", err)
	}
	if err := s.AppendAudit(e2); err != nil {
		t.Fatalf("AppendAudit e2: %v", err)
	}

	got, err := s.ListAudit(100)
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Action != "lifecycle.start" || got[1].Action != "db.exec" {
		t.Errorf("order wrong: got %q,%q", got[0].Action, got[1].Action)
	}
}

func TestListAuditLimit(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "state.bolt"))
	defer s.Close()
	for i := 0; i < 10; i++ {
		_ = s.AppendAudit(AuditEntry{Operator: "local", Action: "x"})
	}
	got, _ := s.ListAudit(3)
	if len(got) != 3 {
		t.Errorf("len = %d, want 3", len(got))
	}
}
