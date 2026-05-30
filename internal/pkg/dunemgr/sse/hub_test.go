package sse

import (
	"testing"
	"time"
)

func TestHubDeliversToSubscriber(t *testing.T) {
	h := NewHub()
	ch, cancel := h.Subscribe("bg/vm-a")
	defer cancel()
	h.Publish("bg/vm-a", Event{Data: "RUNNING"})
	select {
	case ev := <-ch:
		if ev.Data != "RUNNING" {
			t.Errorf("data=%q, want RUNNING", ev.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("no event delivered")
	}
}

func TestHubIsolatesTopics(t *testing.T) {
	h := NewHub()
	ch, cancel := h.Subscribe("bg/vm-a")
	defer cancel()
	h.Publish("bg/vm-b", Event{Data: "X"})
	select {
	case ev := <-ch:
		t.Errorf("leaked cross-topic event: %+v", ev)
	case <-time.After(50 * time.Millisecond):
		// expected: nothing on the other topic
	}
}

func TestHubUnsubscribeRemovesSubscriber(t *testing.T) {
	h := NewHub()
	_, cancel := h.Subscribe("t")
	if h.SubscriberCount("t") != 1 {
		t.Fatalf("count=%d, want 1", h.SubscriberCount("t"))
	}
	cancel()
	if h.SubscriberCount("t") != 0 {
		t.Errorf("count=%d after cancel, want 0", h.SubscriberCount("t"))
	}
	h.Publish("t", Event{Data: "x"})
}

func TestHubPublishDoesNotBlockOnFullSubscriber(t *testing.T) {
	h := NewHub()
	_, cancel := h.Subscribe("t") // never drained
	defer cancel()
	done := make(chan struct{})
	go func() {
		for i := 0; i < subscriberBuffer+10; i++ {
			h.Publish("t", Event{Data: "x"})
		}
		close(done)
	}()
	select {
	case <-done:
		// expected: Publish dropped overflow instead of blocking
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on a full subscriber buffer")
	}
}
