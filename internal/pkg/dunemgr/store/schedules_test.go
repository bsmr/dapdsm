package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func openTempStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestScheduleRoundTrip(t *testing.T) {
	s := openTempStore(t)
	rec := ScheduledShutdown{
		Host: "vm-a", Kind: "Restart", Action: "stop",
		NowUnix: 1000, AtUnix: 1300, ShutdownDurationS: 30,
		BroadcastFrequency: 60, BroadcastDuration: 10, Operator: "local",
	}
	if err := s.PutSchedule(rec); err != nil {
		t.Fatalf("PutSchedule: %v", err)
	}
	got, err := s.GetSchedule("vm-a")
	if err != nil {
		t.Fatalf("GetSchedule: %v", err)
	}
	if got.AtUnix != 1300 || got.Action != "stop" {
		t.Errorf("got=%+v", got)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt not stamped")
	}
}

func TestGetScheduleMissing(t *testing.T) {
	s := openTempStore(t)
	if _, err := s.GetSchedule("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err=%v, want ErrNotFound", err)
	}
}

func TestDeleteAndListSchedules(t *testing.T) {
	s := openTempStore(t)
	_ = s.PutSchedule(ScheduledShutdown{Host: "vm-a", AtUnix: 10})
	_ = s.PutSchedule(ScheduledShutdown{Host: "vm-b", AtUnix: 20})
	all, err := s.ListSchedules()
	if err != nil {
		t.Fatalf("ListSchedules: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("len=%d, want 2", len(all))
	}
	if err := s.DeleteSchedule("vm-a"); err != nil {
		t.Fatalf("DeleteSchedule: %v", err)
	}
	if _, err := s.GetSchedule("vm-a"); !errors.Is(err, ErrNotFound) {
		t.Errorf("vm-a still present: %v", err)
	}
}
