package broadcast

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.muehmer.eu/dapdsm/pkg/domain/store"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// countAuditAction counts audit entries with the given action string.
func countAuditAction(entries []store.AuditEntry, action string) int {
	n := 0
	for _, e := range entries {
		if e.Action == action {
			n++
		}
	}
	return n
}

// TestAnnouncePublishesThenActs: happy path — Wait returns nil, action runs,
// audit has one broadcast.shutdown and zero broadcast.shutdown-cancel.
func TestAnnouncePublishesThenActs(t *testing.T) {
	rr := defaultFakeRunner("publish=ok")
	r := &Runner{Exec: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	var acted bool
	p := AnnounceParams{
		Operator: "op",
		Host:     "vm-a",
		Kind:     "Restart",
		Delay:    time.Minute,
		Now:      1000,
		Wait:     func(_ context.Context, _ time.Duration) error { return nil },
	}
	err := r.Announce(context.Background(), p, func(_ context.Context) error {
		acted = true
		return nil
	})
	if err != nil {
		t.Fatalf("Announce: %v", err)
	}
	if !acted {
		t.Fatal("action was not run")
	}
	entries, _ := r.Store.ListAudit(0)
	shutdowns := countAuditAction(entries, "broadcast.shutdown")
	cancels := countAuditAction(entries, "broadcast.shutdown-cancel")
	if shutdowns != 1 || cancels != 0 {
		t.Fatalf("expected 1 shutdown + 0 cancel, got %d + %d; entries=%+v", shutdowns, cancels, entries)
	}
}

// TestAnnounceCancelsOnContextDone: Wait returns context.Canceled → action
// must NOT run, err must wrap context.Canceled, audit has a shutdown-cancel.
func TestAnnounceCancelsOnContextDone(t *testing.T) {
	rr := defaultFakeRunner("publish=ok")
	r := &Runner{Exec: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	var acted bool
	p := AnnounceParams{
		Operator: "op",
		Host:     "vm-a",
		Kind:     "Restart",
		Delay:    time.Minute,
		Now:      1000,
		Wait:     func(_ context.Context, _ time.Duration) error { return context.Canceled },
	}
	err := r.Announce(context.Background(), p, func(_ context.Context) error {
		acted = true
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if acted {
		t.Fatal("action ran despite cancellation")
	}
	entries, _ := r.Store.ListAudit(0)
	if countAuditAction(entries, "broadcast.shutdown-cancel") == 0 {
		t.Fatalf("expected a shutdown-cancel broadcast; entries=%+v", entries)
	}
}

// TestAnnounceCancelsOnActionError: Wait returns nil, action returns error →
// err must wrap the action error, audit has a shutdown-cancel.
func TestAnnounceCancelsOnActionError(t *testing.T) {
	rr := defaultFakeRunner("publish=ok")
	r := &Runner{Exec: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	boom := errors.New("boom")
	p := AnnounceParams{
		Operator: "op",
		Host:     "vm-a",
		Kind:     "Maintenance",
		Delay:    time.Second,
		Now:      1000,
		Wait:     func(_ context.Context, _ time.Duration) error { return nil },
	}
	err := r.Announce(context.Background(), p, func(_ context.Context) error { return boom })
	if !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}
	entries, _ := r.Store.ListAudit(0)
	if countAuditAction(entries, "broadcast.shutdown-cancel") == 0 {
		t.Fatalf("expected a shutdown-cancel after action error; entries=%+v", entries)
	}
}
