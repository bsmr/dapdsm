package sse

import (
	"fmt"
	"net/http"
	"strings"
)

// ServeStream subscribes the request to topic and streams events as
// text/event-stream until the client disconnects (request context
// cancelled). The caller is responsible for auth — wrap the enclosing
// handler in the usual middleware. Requires that w implements
// http.Flusher (the stdlib server's ResponseWriter does; auth/Origin
// middleware passes it through unwrapped).
func ServeStream(w http.ResponseWriter, r *http.Request, hub *Hub, topic string) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering

	ch, cancel := hub.Subscribe(topic)
	defer cancel()

	// Open the stream so the browser's EventSource fires "open" and any
	// intermediary flushes headers immediately.
	fmt.Fprint(w, ": connected\n\n")
	fl.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-ch:
			if ev.Name != "" {
				fmt.Fprintf(w, "event: %s\n", ev.Name)
			}
			for _, line := range strings.Split(ev.Data, "\n") {
				fmt.Fprintf(w, "data: %s\n", line)
			}
			fmt.Fprint(w, "\n")
			fl.Flush()
		}
	}
}
