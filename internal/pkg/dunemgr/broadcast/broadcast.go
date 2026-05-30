// Package broadcast publishes Funcom in-game messages (notice banner
// + scheduled-shutdown countdown) by piping an Erlang expression to
// `rabbitmqctl eval` inside the RabbitMQ pod over SSH+kubectl-exec.
package broadcast

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
)

// EncodeEnvelope wraps an inner JSON ServerCommand payload in the
// outer envelope format expected by Funcom's RabbitMQ-side parser:
//
//	{"Version": 2, "AuthToken": "<token>", "MessageContent": "<inner-as-string>"}
//
// then base64-encodes the result.
func EncodeEnvelope(inner []byte, token string) string {
	outer := struct {
		Version        int    `json:"Version"`
		AuthToken      string `json:"AuthToken"`
		MessageContent string `json:"MessageContent"`
	}{
		Version:        2,
		AuthToken:      token,
		MessageContent: string(inner),
	}
	b, _ := json.Marshal(outer)
	return base64.StdEncoding.EncodeToString(b)
}

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

// Constants matching the Funcom-side RabbitMQ topology.
const (
	exchange   = "heartbeats"
	routingKey = "notifications"
	userID     = "fls"
	appID      = "fls_backend"
)

var safeLabelRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

// BuildErlangPublish renders the Erlang expression that
// `rabbitmqctl eval` executes inside the MQ pod. The expression
// base64-decodes the envelope, builds a basic message with
// app/user identity attributed to Funcom's FLS backend, and
// publishes to exchange/routingKey above. `label` is a free-form
// short identifier that ends up in the server-side log line; it is
// sanitized against an allowlist to prevent Erlang-string injection.
func BuildErlangPublish(payloadB64, label string) string {
	if !safeLabelRE.MatchString(label) {
		label = "smgmt"
	}
	return fmt.Sprintf(
		"Outer = base64:decode(<<\"%s\">>),\n"+
			"XName = rabbit_misc:r(<<\"/\">>, exchange, <<\"%s\">>),\n"+
			"X = rabbit_exchange:lookup_or_die(XName),\n"+
			"MsgId = list_to_binary(\"dunemgr-\" ++ \"%s\" ++ \"-\" ++ integer_to_list(erlang:system_time(millisecond))),\n"+
			"P = {list_to_atom(\"P_basic\"), <<\"Content\">>, undefined, [], undefined, undefined, undefined, undefined, undefined, MsgId, undefined, undefined, <<\"%s\">>, <<\"%s\">>, undefined},\n"+
			"Content = rabbit_basic:build_content(P, Outer),\n"+
			"{ok, Msg} = rabbit_basic:message(XName, <<\"%s\">>, Content),\n"+
			"Result = rabbit_queue_type:publish_at_most_once(X, Msg),\n"+
			"io:format(\"publish=~p exchange=%s routing=%s app_id=%s user_id=%s label=%s~n\", [Result]).\n",
		payloadB64, exchange, label, userID, appID, routingKey,
		exchange, routingKey, appID, userID, label,
	)
}
