// Package sse provides a tiny in-process publish/subscribe hub and an
// http.Flusher-based handler for Server-Sent Events. Topics are opaque
// strings (dunemgr uses "<channel>/<host>", e.g. "bg/vm-a").
package sse

import "sync"

// subscriberBuffer is the per-subscriber channel depth. A consumer that
// falls this far behind drops intermediate events (Publish never blocks).
const subscriberBuffer = 16

// Event is one SSE frame. Name maps to the SSE "event:" field (empty =>
// the default "message" event, which fires EventSource.onmessage). Data
// maps to "data:"; multi-line Data is split across multiple data: lines
// by the handler.
type Event struct {
	Name string
	Data string
}

// Hub is a concurrency-safe topic->subscribers registry.
type Hub struct {
	mu     sync.Mutex
	topics map[string]map[chan Event]struct{}
}

// NewHub returns an empty Hub.
func NewHub() *Hub {
	return &Hub{topics: map[string]map[chan Event]struct{}{}}
}

// Subscribe registers a new subscriber on topic and returns its receive
// channel plus an idempotent unsubscribe func. Always call the returned
// func (defer it) to avoid leaking the subscriber.
func (h *Hub) Subscribe(topic string) (<-chan Event, func()) {
	ch := make(chan Event, subscriberBuffer)
	h.mu.Lock()
	subs := h.topics[topic]
	if subs == nil {
		subs = map[chan Event]struct{}{}
		h.topics[topic] = subs
	}
	subs[ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			if subs := h.topics[topic]; subs != nil {
				delete(subs, ch)
				if len(subs) == 0 {
					delete(h.topics, topic)
				}
			}
			h.mu.Unlock()
		})
	}
	return ch, cancel
}

// Publish delivers ev to every current subscriber of topic. Delivery is
// non-blocking: a subscriber whose buffer is full silently drops ev.
func (h *Hub) Publish(topic string, ev Event) {
	h.mu.Lock()
	subs := make([]chan Event, 0, len(h.topics[topic]))
	for ch := range h.topics[topic] {
		subs = append(subs, ch)
	}
	h.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- ev:
		default: // slow consumer — drop
		}
	}
}

// SubscriberCount returns the number of live subscribers on topic.
func (h *Hub) SubscriberCount(topic string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.topics[topic])
}
