package tui

import (
	"encoding/json"
	"testing"
	"time"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/sse"
)

func TestBridgeForwardsBGFrameAsPollMsg(t *testing.T) {
	hub := sse.NewHub()
	out, cancel := subscribeHosts(hub, []string{"vm-a"})
	defer cancel()

	data, _ := json.Marshal(map[string]any{"state": "RUNNING", "ready": 2, "total": 2})
	hub.Publish("bg/vm-a", sse.Event{Data: string(data)})

	select {
	case msg := <-out:
		if msg.host != "vm-a" || msg.kind != pollBG || msg.bgState != "RUNNING" || msg.ready != 2 {
			t.Fatalf("unexpected pollMsg: %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("bridge did not forward the bg frame")
	}
}

func TestBridgeForwardsHealthFrame(t *testing.T) {
	hub := sse.NewHub()
	out, cancel := subscribeHosts(hub, []string{"vm-a"})
	defer cancel()

	data, _ := json.Marshal(map[string]any{"ok": true, "probedAt": "now"})
	hub.Publish("health/vm-a", sse.Event{Data: string(data)})

	select {
	case msg := <-out:
		if msg.kind != pollHealth || !msg.reachable {
			t.Fatalf("unexpected health pollMsg: %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("bridge did not forward the health frame")
	}
}
