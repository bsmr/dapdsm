// internal/pkg/dunemgr/server/events_test.go
package server

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/auth"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/sse"
)

func TestEventsBGStreamsPublishedEvent(t *testing.T) {
	hub := sse.NewHub()
	srv := newTestServerWithHub(t, hub)
	ts := httptest.NewServer(srv.Handler)
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/events/vm-a/bg", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}

	deadline := time.Now().Add(2 * time.Second)
	for hub.SubscriberCount("bg/vm-a") == 0 {
		if time.Now().After(deadline) {
			t.Fatal("route never subscribed to bg/vm-a")
		}
		time.Sleep(5 * time.Millisecond)
	}
	hub.Publish("bg/vm-a", sse.Event{Data: "RUNNING"})

	br := bufio.NewReader(resp.Body)
	found := false
	for i := 0; i < 50; i++ {
		line, err := br.ReadString('\n')
		if err != nil {
			break
		}
		if strings.Contains(line, "data: RUNNING") {
			found = true
			break
		}
	}
	if !found {
		t.Error("bg event not streamed")
	}
}

func TestEventsRequireAuth(t *testing.T) {
	srv := newTestServerWithHub(t, sse.NewHub())
	req := httptest.NewRequest("GET", "/events/vm-a/bg", nil)
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Errorf("unauthenticated /events returned 200")
	}
}

func TestEventsUnknownChannel404(t *testing.T) {
	srv := newTestServerWithHub(t, sse.NewHub())
	req := httptest.NewRequest("GET", "/events/vm-a/frobnicate", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Errorf("unknown channel returned 200")
	}
}
