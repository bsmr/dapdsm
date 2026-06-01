// Package broadcast publishes Funcom in-game messages (notice banner
// + scheduled-shutdown countdown) by piping an Erlang expression to
// `rabbitmqctl eval` inside the RabbitMQ pod over SSH+kubectl-exec.
//
// The wire transport is provided by the mq package; this package owns
// the Funcom-specific payload builders and the Runner that wires them
// together.
package broadcast

import (
	"encoding/json"
)

// NoticePayload returns a JSON inner payload for a Generic
// (banner-style) broadcast.
func NoticePayload(title, body string, durationSecs int) string {
	type entry struct {
		Key   string `json:"Key"`
		Title string `json:"Title"`
		Body  string `json:"Body"`
	}
	payload := map[string]any{
		"ServerCommand": "ServiceBroadcast",
		"BroadcastType": "Generic",
		"BroadcastPayload": map[string]any{
			"BroadcastDuration": durationSecs,
			"LocalizedText": []entry{
				{Key: "en", Title: title, Body: body},
				{Key: "en-US", Title: title, Body: body},
			},
		},
	}
	b, _ := json.Marshal(payload)
	return string(b)
}

// ShutdownAnnounce describes a scheduled-shutdown countdown.
type ShutdownAnnounce struct {
	// Kind is one of "Restart", "Maintenance", "Update".
	Kind string
	// AtUnix is when the shutdown will actually happen.
	AtUnix int64
	// NowUnix is the moment the announce is published.
	NowUnix int64
	// ShutdownDurationS is how long the shutdown itself is expected
	// to take (visible to clients).
	ShutdownDurationS int
	// BroadcastFrequency: how often the client repeats the banner
	// during the countdown (seconds).
	BroadcastFrequency int
	// BroadcastDuration: how long each banner appears (seconds).
	BroadcastDuration int
}

// ShutdownAnnouncePayload builds the inner payload for a scheduled-
// shutdown countdown broadcast.
func ShutdownAnnouncePayload(a ShutdownAnnounce) string {
	payload := map[string]any{
		"ServerCommand": "ServiceBroadcast",
		"BroadcastType": "ServerShutdown",
		"BroadcastPayload": map[string]any{
			"ShutdownType":       a.Kind,
			"DateTimestamp":      a.NowUnix,
			"ShutdownDuration":   a.ShutdownDurationS,
			"ShutdownTimestamp":  a.AtUnix,
			"BroadcastFrequency": a.BroadcastFrequency,
			"BroadcastDuration":  a.BroadcastDuration,
		},
	}
	b, _ := json.Marshal(payload)
	return string(b)
}

// ShutdownCancelPayload builds the inner payload that cancels an
// in-flight scheduled shutdown.
func ShutdownCancelPayload() string {
	payload := map[string]any{
		"ServerCommand": "ServiceBroadcast",
		"BroadcastType": "ServerShutdown",
		"BroadcastPayload": map[string]any{
			"ShouldCancel": true,
		},
	}
	b, _ := json.Marshal(payload)
	return string(b)
}
