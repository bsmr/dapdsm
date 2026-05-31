package tui

import (
	"encoding/json"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/sse"
)

// bgFrame mirrors the JSON the poller publishes on "bg/<host>".
type bgFrame struct {
	State string `json:"state"`
	Ready int    `json:"ready"`
	Total int    `json:"total"`
	Error string `json:"error"`
}

// healthFrame mirrors the JSON the poller publishes on "health/<host>".
type healthFrame struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

// actionFrame mirrors the JSON the poller publishes on "actions/<host>".
type actionFrame struct {
	Action string `json:"action"`
	Result string `json:"result"`
}

// subscribeHosts subscribes to the bg/health/actions topics of each host and
// returns a single channel of decoded pollMsgs plus a cancel func that
// unsubscribes everything. The output channel is buffered; a slow consumer
// drops frames (status reconciles on the next poll tick).
func subscribeHosts(hub *sse.Hub, hosts []string) (<-chan pollMsg, func()) {
	out := make(chan pollMsg, 64)
	var cancels []func()
	done := make(chan struct{})

	forward := func(ch <-chan sse.Event, decode func(string) (pollMsg, bool)) {
		for {
			select {
			case <-done:
				return
			case ev, ok := <-ch:
				if !ok {
					return
				}
				if msg, send := decode(ev.Data); send {
					select {
					case out <- msg:
					default: // slow consumer — drop
					}
				}
			}
		}
	}

	for _, h := range hosts {
		host := h
		bgCh, c1 := hub.Subscribe("bg/" + host)
		healthCh, c2 := hub.Subscribe("health/" + host)
		actCh, c3 := hub.Subscribe("actions/" + host)
		cancels = append(cancels, c1, c2, c3)

		go forward(bgCh, func(d string) (pollMsg, bool) {
			var f bgFrame
			if json.Unmarshal([]byte(d), &f) != nil {
				return pollMsg{}, false
			}
			return pollMsg{host: host, kind: pollBG, bgState: f.State, ready: f.Ready, total: f.Total, err: f.Error}, true
		})
		go forward(healthCh, func(d string) (pollMsg, bool) {
			var f healthFrame
			if json.Unmarshal([]byte(d), &f) != nil {
				return pollMsg{}, false
			}
			return pollMsg{host: host, kind: pollHealth, reachable: f.OK, err: f.Error}, true
		})
		go forward(actCh, func(d string) (pollMsg, bool) {
			var f actionFrame
			if json.Unmarshal([]byte(d), &f) != nil {
				return pollMsg{}, false
			}
			return pollMsg{host: host, kind: pollAction, action: f.Action, result: f.Result}, true
		})
	}

	var once bool
	cancel := func() {
		if once {
			return
		}
		once = true
		close(done)
		for _, c := range cancels {
			c()
		}
	}
	return out, cancel
}
