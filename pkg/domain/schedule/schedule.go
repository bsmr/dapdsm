// Package schedule owns countdown-shutdowns: it announces a Funcom
// shutdown (the game renders the countdown banners) and arms an
// in-process timer that runs the real lifecycle verb at the deadline.
// Pending shutdowns persist to the store's schedules bucket and are
// re-armed on startup.
package schedule

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.muehmer.eu/dapdsm/pkg/domain/broadcast"
	"go.muehmer.eu/dapdsm/pkg/domain/lifecycle"
	"go.muehmer.eu/dapdsm/pkg/domain/store"
)

// Request is the operator-supplied parameters for a countdown.
type Request struct {
	Kind               string           // Funcom ShutdownType: Restart|Maintenance|Update
	LeadSecs           int              // seconds from now until the lifecycle verb runs
	Action             lifecycle.Action // stop | restart (validated)
	ShutdownDurationS  int
	BroadcastFrequency int
	BroadcastDuration  int
}

// EventPublisher receives schedule action events. Implemented by the host
// process (e.g. an sse.Hub adapter); nil disables publishing.
type EventPublisher interface {
	Publish(topic, data string)
}

// Manager arms per-host countdown timers and persists them.
type Manager struct {
	bc    *broadcast.Runner
	lc    *lifecycle.Runner
	store *store.Store
	pub   EventPublisher // optional; nil ok

	mu     sync.Mutex
	timers map[string]*time.Timer
}

// NewManager wires the runners + store (+ optional EventPublisher).
func NewManager(bc *broadcast.Runner, lc *lifecycle.Runner, st *store.Store, pub EventPublisher) *Manager {
	return &Manager{bc: bc, lc: lc, store: st, pub: pub, timers: map[string]*time.Timer{}}
}

// Schedule announces a shutdown, persists it, and arms the timer.
func (m *Manager) Schedule(ctx context.Context, operator, host string, req Request) error {
	if req.Action != lifecycle.ActionStop && req.Action != lifecycle.ActionRestart {
		return fmt.Errorf("schedule: action %q: only stop|restart can be scheduled", req.Action)
	}
	if req.LeadSecs <= 0 {
		return fmt.Errorf("schedule: lead seconds must be > 0, got %d", req.LeadSecs)
	}
	now := time.Now().Unix()
	at := now + int64(req.LeadSecs)

	if _, err := m.bc.PublishShutdownAnnounce(ctx, operator, host, broadcast.ShutdownAnnounce{
		Kind:               req.Kind,
		NowUnix:            now,
		AtUnix:             at,
		ShutdownDurationS:  req.ShutdownDurationS,
		BroadcastFrequency: req.BroadcastFrequency,
		BroadcastDuration:  req.BroadcastDuration,
	}); err != nil {
		return fmt.Errorf("schedule: announce: %w", err)
	}

	rec := store.ScheduledShutdown{
		Host: host, Kind: req.Kind, Action: string(req.Action),
		NowUnix: now, AtUnix: at, ShutdownDurationS: req.ShutdownDurationS,
		BroadcastFrequency: req.BroadcastFrequency, BroadcastDuration: req.BroadcastDuration,
		Operator: operator,
	}
	if err := m.store.PutSchedule(rec); err != nil {
		return fmt.Errorf("schedule: persist: %w", err)
	}
	m.arm(host, time.Duration(req.LeadSecs)*time.Second)
	m.notify(host, "shutdown.scheduled", req.Action, operator)
	return nil
}

// Cancel publishes a shutdown-cancel, stops the timer, deletes the record.
//
// Race note: time.Timer.Stop on an AfterFunc timer does not wait for an
// already-started timer goroutine. If executeShutdown has just begun when
// Cancel runs, the two race on the store — but it is safe: whichever
// DeleteSchedule wins, the loser's GetSchedule returns ErrNotFound and
// no-ops. The worst case is a redundant shutdown-cancel publish.
func (m *Manager) Cancel(ctx context.Context, operator, host string) error {
	if _, err := m.bc.PublishShutdownCancel(ctx, operator, host); err != nil {
		return fmt.Errorf("cancel: announce: %w", err)
	}
	m.disarm(host)
	if err := m.store.DeleteSchedule(host); err != nil {
		return fmt.Errorf("cancel: delete: %w", err)
	}
	m.notify(host, "shutdown.cancelled", "", operator)
	return nil
}

// Pending reports whether host has a live (armed) countdown.
func (m *Manager) Pending(host string) (store.ScheduledShutdown, bool) {
	m.mu.Lock()
	_, armed := m.timers[host]
	m.mu.Unlock()
	if !armed {
		return store.ScheduledShutdown{}, false
	}
	rec, err := m.store.GetSchedule(host)
	if err != nil {
		return store.ScheduledShutdown{}, false
	}
	return rec, true
}

// Rearm reloads persisted shutdowns and re-arms timers. Future
// deadlines get a timer for the remaining duration; past-due ones
// execute immediately (best-effort catch-up after a restart).
func (m *Manager) Rearm(ctx context.Context) {
	all, err := m.store.ListSchedules()
	if err != nil {
		return
	}
	now := time.Now().Unix()
	for _, rec := range all {
		if rec.AtUnix <= now {
			m.executeShutdown(ctx, rec.Host)
			continue
		}
		m.arm(rec.Host, time.Duration(rec.AtUnix-now)*time.Second)
	}
}

// arm installs (replacing any prior) an AfterFunc timer for host.
func (m *Manager) arm(host string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t := m.timers[host]; t != nil {
		t.Stop()
	}
	m.timers[host] = time.AfterFunc(d, func() {
		m.executeShutdown(context.Background(), host)
	})
}

// disarm stops and forgets host's timer.
func (m *Manager) disarm(host string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t := m.timers[host]; t != nil {
		t.Stop()
		delete(m.timers, host)
	}
}

// executeShutdown runs the persisted lifecycle verb for host, then
// cleans up the record + timer. Called by the timer or by Rearm.
func (m *Manager) executeShutdown(ctx context.Context, host string) {
	rec, err := m.store.GetSchedule(host)
	if err != nil {
		m.disarm(host)
		return
	}
	action, verr := lifecycle.ValidateAction(rec.Action)
	if verr == nil {
		_, _ = m.lc.Run(ctx, "scheduler", host, action)
	}
	_ = m.store.DeleteSchedule(host)
	m.disarm(host)
	m.notify(host, "shutdown.executed", lifecycle.Action(rec.Action), rec.Operator)
}

// notify publishes a best-effort actions event (no-op if pub nil).
func (m *Manager) notify(host, action string, verb lifecycle.Action, operator string) {
	if m.pub == nil {
		return
	}
	data, _ := json.Marshal(map[string]string{
		"action": action, "result": string(verb), "operator": operator,
	})
	m.pub.Publish("actions/"+host, string(data))
}
