package sse

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.muehmer.eu/dapdsm/internal/pkg/battlegroup"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
)

func drain(t *testing.T, ch <-chan Event) (Event, bool) {
	t.Helper()
	select {
	case ev := <-ch:
		return ev, true
	case <-time.After(time.Second):
		return Event{}, false
	}
}

func newTestPoller(hub *Hub, snap store.StatusSnapshot, audit []store.AuditEntry) *Poller {
	return &Poller{
		Hub:   hub,
		Hosts: func() ([]string, error) { return []string{"vm-a"}, nil },
		Probe: func(_ context.Context, host string) (store.StatusSnapshot, error) {
			s := snap
			s.Host = host
			return s, nil
		},
		Audit: func() ([]store.AuditEntry, error) { return audit, nil },
	}
}

func TestPollerPublishesBGOnFirstTick(t *testing.T) {
	hub := NewHub()
	p := newTestPoller(hub, store.StatusSnapshot{BGState: "RUNNING", PodReady: 3, PodTotal: 3}, nil)
	ch, cancel := hub.Subscribe("bg/vm-a")
	defer cancel()
	st := newPollState()
	p.Tick(context.Background(), st)
	ev, ok := drain(t, ch)
	if !ok || !strings.Contains(ev.Data, "RUNNING") {
		t.Errorf("bg event=%+v ok=%v", ev, ok)
	}
}

func TestPollerSuppressesUnchangedBG(t *testing.T) {
	hub := NewHub()
	p := newTestPoller(hub, store.StatusSnapshot{BGState: "RUNNING", PodReady: 3, PodTotal: 3}, nil)
	ch, cancel := hub.Subscribe("bg/vm-a")
	defer cancel()
	st := newPollState()
	p.Tick(context.Background(), st) // publishes
	<-ch
	p.Tick(context.Background(), st) // unchanged → no publish
	if _, ok := drain(t, ch); ok {
		t.Error("second tick published despite unchanged bg")
	}
}

func TestPollerPublishesHealthOnFirstTick(t *testing.T) {
	hub := NewHub()
	p := newTestPoller(hub, store.StatusSnapshot{BGState: "RUNNING", PodReady: 2, PodTotal: 2}, nil)
	ch, cancel := hub.Subscribe("health/vm-a")
	defer cancel()
	st := newPollState()
	p.Tick(context.Background(), st)
	ev, ok := drain(t, ch)
	if !ok || !strings.Contains(ev.Data, `"ok":true`) {
		t.Errorf("health event=%+v ok=%v", ev, ok)
	}
}

func TestPollerHealthReflectsError(t *testing.T) {
	hub := NewHub()
	p := newTestPoller(hub, store.StatusSnapshot{BGState: "UNKNOWN", Error: "boom"}, nil)
	ch, cancel := hub.Subscribe("health/vm-a")
	defer cancel()
	st := newPollState()
	p.Tick(context.Background(), st)
	ev, ok := drain(t, ch)
	if !ok || !strings.Contains(ev.Data, `"ok":false`) || !strings.Contains(ev.Data, "boom") {
		t.Errorf("health event=%+v ok=%v", ev, ok)
	}
}

func TestPollerPublishesNewAuditEntryForHost(t *testing.T) {
	hub := NewHub()
	base := []store.AuditEntry{{Host: "vm-a", Action: "old", Result: "ok"}}
	p := newTestPoller(hub, store.StatusSnapshot{BGState: "RUNNING"}, base)
	ch, cancel := hub.Subscribe("actions/vm-a")
	defer cancel()
	st := newPollState()
	p.Tick(context.Background(), st) // captures audit baseline, no replay
	if _, ok := drain(t, ch); ok {
		t.Fatal("first tick replayed existing audit history")
	}
	p.Audit = func() ([]store.AuditEntry, error) {
		return append(base, store.AuditEntry{Host: "vm-a", Action: "lifecycle.start", Result: "ok", Operator: "local"}), nil
	}
	p.Tick(context.Background(), st)
	ev, ok := drain(t, ch)
	if !ok || !strings.Contains(ev.Data, "lifecycle.start") {
		t.Errorf("actions event=%+v ok=%v", ev, ok)
	}
}

func TestPollerPrunesRemovedHost(t *testing.T) {
	hub := NewHub()
	hosts := []string{"vm-a"}
	p := &Poller{
		Hub:   hub,
		Hosts: func() ([]string, error) { return hosts, nil },
		Probe: func(_ context.Context, h string) (store.StatusSnapshot, error) {
			return store.StatusSnapshot{Host: h, BGState: "RUNNING"}, nil
		},
		Audit: func() ([]store.AuditEntry, error) { return nil, nil },
	}
	st := newPollState()
	p.Tick(context.Background(), st) // records vm-a
	hosts = []string{}               // vm-a removed
	p.Tick(context.Background(), st)
	if _, ok := st.lastBG["vm-a"]; ok {
		t.Error("lastBG still holds removed host vm-a")
	}
	if _, ok := st.lastHealth["vm-a"]; ok {
		t.Error("lastHealth still holds removed host vm-a")
	}
}

func TestPollerPublishesOnServerDetailChange(t *testing.T) {
	hub := NewHub()
	base := store.StatusSnapshot{
		BGState: "RUNNING", PodReady: 1, PodTotal: 1,
		Detail: battlegroup.Status{Servers: []battlegroup.ServerStatus{
			{Map: "Overmap", Phase: "Running", Ready: true, Restarts: 0},
		}},
	}
	p := newTestPoller(hub, base, nil)
	ch, cancel := hub.Subscribe("bg/vm-a")
	defer cancel()
	st := newPollState()
	p.Tick(context.Background(), st) // first publish
	<-ch

	// Same state/ready/total, but Restarts changed → must republish.
	changed := base
	changed.Detail = battlegroup.Status{Servers: []battlegroup.ServerStatus{
		{Map: "Overmap", Phase: "Running", Ready: true, Restarts: 1},
	}}
	p.Probe = func(_ context.Context, host string) (store.StatusSnapshot, error) {
		s := changed
		s.Host = host
		return s, nil
	}
	p.Tick(context.Background(), st)
	if _, ok := drain(t, ch); !ok {
		t.Error("server-detail change did not republish")
	}
}
