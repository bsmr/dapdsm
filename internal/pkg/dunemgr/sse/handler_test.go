package sse

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func contextWithCancel() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

func TestServeStreamSetsEventStreamHeaders(t *testing.T) {
	hub := NewHub()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeStream(w, r, hub, "bg/vm-a")
	}))
	defer srv.Close()

	ctx, cancel := contextWithCancel()
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type=%q, want text/event-stream", ct)
	}
}

func TestServeStreamDeliversPublishedEvent(t *testing.T) {
	hub := NewHub()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeStream(w, r, hub, "bg/vm-a")
	}))
	defer srv.Close()

	ctx, cancel := contextWithCancel()
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	deadline := time.Now().Add(2 * time.Second)
	for hub.SubscriberCount("bg/vm-a") == 0 {
		if time.Now().After(deadline) {
			t.Fatal("handler never subscribed")
		}
		time.Sleep(5 * time.Millisecond)
	}
	hub.Publish("bg/vm-a", Event{Data: "RUNNING"})

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
		t.Error("published event not received on the stream")
	}
}
