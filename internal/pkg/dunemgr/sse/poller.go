package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
)

// DefaultInterval is the v1 poll cadence.
const DefaultInterval = 5 * time.Second

// Poller periodically probes each known host and publishes only the
// values that changed since the previous tick. All external interactions
// are injected funcs so the loop is testable without SSH/kubectl.
type Poller struct {
	Hub      *Hub
	Hosts    func() ([]string, error)
	Probe    func(ctx context.Context, host string) (store.StatusSnapshot, error)
	Tunnel   func(host string) bool
	Audit    func() ([]store.AuditEntry, error)
	Interval time.Duration // 0 => DefaultInterval
}

// pollState carries the previous tick's values for change detection.
type pollState struct {
	lastBG     map[string]string
	lastTunnel map[string]bool
	auditSeen  int // -1 = uninitialised
}

func newPollState() *pollState {
	return &pollState{
		lastBG:     map[string]string{},
		lastTunnel: map[string]bool{},
		auditSeen:  -1,
	}
}

type bgEvent struct {
	State string `json:"state"`
	Ready int    `json:"ready"`
	Total int    `json:"total"`
	Error string `json:"error,omitempty"`
}

type tunnelEvent struct {
	Up bool `json:"up"`
}

type actionEvent struct {
	Action   string `json:"action"`
	Result   string `json:"result"`
	Operator string `json:"operator"`
}

// Tick runs one poll pass and publishes any changes into the Hub.
func (p *Poller) Tick(ctx context.Context, st *pollState) {
	hosts, err := p.Hosts()
	if err != nil {
		return
	}

	for _, host := range hosts {
		snap, _ := p.Probe(ctx, host)
		detail, _ := json.Marshal(snap.Detail.Servers)
		key := fmt.Sprintf("%s|%d|%d|%s|%s", snap.BGState, snap.PodReady, snap.PodTotal, snap.Error, detail)
		if st.lastBG[host] != key {
			st.lastBG[host] = key
			data, _ := json.Marshal(bgEvent{State: snap.BGState, Ready: snap.PodReady, Total: snap.PodTotal, Error: snap.Error})
			p.Hub.Publish("bg/"+host, Event{Data: string(data)})
		}

		up := p.Tunnel(host)
		if prev, ok := st.lastTunnel[host]; !ok || prev != up {
			st.lastTunnel[host] = up
			data, _ := json.Marshal(tunnelEvent{Up: up})
			p.Hub.Publish("tunnel/"+host, Event{Data: string(data)})
		}
	}

	all, err := p.Audit()
	if err == nil {
		if st.auditSeen < 0 {
			st.auditSeen = len(all)
		} else if len(all) > st.auditSeen {
			for _, e := range all[st.auditSeen:] {
				data, _ := json.Marshal(actionEvent{Action: e.Action, Result: e.Result, Operator: e.Operator})
				p.Hub.Publish("actions/"+e.Host, Event{Data: string(data)})
			}
			st.auditSeen = len(all)
		}
	}

	// Prune state for hosts no longer present so the maps don't grow
	// unbounded across very long uptimes with host churn.
	live := make(map[string]struct{}, len(hosts))
	for _, h := range hosts {
		live[h] = struct{}{}
	}
	for h := range st.lastBG {
		if _, ok := live[h]; !ok {
			delete(st.lastBG, h)
		}
	}
	for h := range st.lastTunnel {
		if _, ok := live[h]; !ok {
			delete(st.lastTunnel, h)
		}
	}
}

// Run ticks every p.Interval until ctx is cancelled. Wiring-only; the
// per-tick logic is covered by Tick's tests.
func (p *Poller) Run(ctx context.Context) {
	interval := p.Interval
	if interval <= 0 {
		interval = DefaultInterval
	}
	st := newPollState()
	t := time.NewTicker(interval)
	defer t.Stop()
	p.Tick(ctx, st)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.Tick(ctx, st)
		}
	}
}
