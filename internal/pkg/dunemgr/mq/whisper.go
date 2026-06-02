package mq

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
)

// flsRE matches a Funcom FLS id: hex, possibly digit-first (e.g. 127AC6307755DB02).
var flsRE = regexp.MustCompile(`^[0-9A-Fa-f]{1,64}$`)

const whisperExchange = "chat.whispers"

// buildWhisperChatJSON renders the inner Funcom chat message for a whisper.
// Field names follow the C++ FChatMessageData convention (m_ prefix stripped).
// spoof controls whether bUseSpoofedUserName is set; fromName populates
// SpoofedUserNameFrom.AuthorName.
func buildWhisperChatJSON(toName, fromName, message string, spoof bool) string {
	msg := struct {
		ID           string `json:"Id"`
		ChannelType  string `json:"ChannelType"`
		FuncomIdFrom string `json:"FuncomIdFrom"`
		UserNameTo   string `json:"UserNameTo"`
		Message      struct {
			Body string `json:"Body"`
		} `json:"Message"`
		UseSpoofedUserName  bool `json:"bUseSpoofedUserName"`
		SpoofedUserNameFrom struct {
			AuthorName string `json:"AuthorName"`
		} `json:"SpoofedUserNameFrom"`
	}{
		ID:                 "dunemgr-whisper",
		ChannelType:        "ETextChatChannelType::Whispers",
		UserNameTo:         toName,
		UseSpoofedUserName: spoof,
	}
	msg.Message.Body = message
	msg.SpoofedUserNameFrom.AuthorName = fromName
	b, _ := json.Marshal(msg)
	return string(b)
}

// EncodeWhisperEnvelope wraps the chat JSON in the FCourierMessageContent
// envelope and base64-encodes the result. No {Version,AuthToken,MessageContent}
// ServerCommand wrapper — whispers ride the courier directly (exchange
// chat.whispers), not the heartbeats/ServerCommand path.
func EncodeWhisperEnvelope(chatJSON string) string {
	outer := struct {
		Content string `json:"Content"`
		Type    string `json:"Type"`
	}{Content: chatJSON, Type: "ECourierMessageType::TextChat"}
	b, _ := json.Marshal(outer)
	return base64.StdEncoding.EncodeToString(b)
}

// BuildErlangWhisper renders the rabbitmqctl-eval expression that publishes a
// whisper. Differences from BuildErlangPublish (reconciled against
// dune-server-service/src/admin/mq.rs:146-172):
//
//   - exchange = chat.whispers (not heartbeats)
//   - routing key = targetFLS (recipient's FLS ID, not "notifications")
//   - P_basic type field (position 11) = <<"text_chat">> (not undefined)
//   - P_basic user_id field (position 12) = senderFLS (not <<"fls">>)
//   - app_id field (position 13) = <<"fls_backend">> (unchanged)
//
// Both targetFLS and senderFLS are sanitized against flsRE to prevent
// Erlang string injection; an empty string is substituted on rejection.
func BuildErlangWhisper(payloadB64, targetFLS, senderFLS string) string {
	if !flsRE.MatchString(targetFLS) {
		targetFLS = ""
	}
	if !flsRE.MatchString(senderFLS) {
		senderFLS = ""
	}
	return fmt.Sprintf(
		"Outer = base64:decode(<<\"%s\">>),\n"+
			"XName = rabbit_misc:r(<<\"/\">>, exchange, <<\"%s\">>),\n"+
			"X = rabbit_exchange:lookup_or_die(XName),\n"+
			"MsgId = list_to_binary(\"dunemgr-whisper-\" ++ integer_to_list(erlang:system_time(millisecond))),\n"+
			"P = {list_to_atom(\"P_basic\"), <<\"Content\">>, undefined, [], undefined, undefined, undefined, undefined, undefined, MsgId, undefined, <<\"text_chat\">>, <<\"%s\">>, <<\"%s\">>, undefined},\n"+
			"Content = rabbit_basic:build_content(P, Outer),\n"+
			"{ok, Msg} = rabbit_basic:message(XName, <<\"%s\">>, Content),\n"+
			"Result = rabbit_queue_type:publish_at_most_once(X, Msg),\n"+
			"io:format(\"publish=~p exchange=%s routing=%s type=text_chat~n\", [Result]).\n",
		payloadB64, whisperExchange, senderFLS, appID, targetFLS,
		whisperExchange, targetFLS,
	)
}

// PublishWhisper sends a private chat message to targetFLS on host. toName is
// the recipient display name; fromName is the spoofed author shown to the
// recipient (empty → bUseSpoofedUserName=false); senderFLS is the AMQP
// user_id field (may be empty — rejected by flsRE → blank).
//
// targetFLS must be a valid hex FLS ID; an error is returned immediately if
// it fails validation so the whisper is never silently dropped.
//
// Audit captures operator/host/targetFLS and outcome; the message body is
// never written to the audit log.
func (p *Publisher) PublishWhisper(ctx context.Context, operator, host, targetFLS, toName, fromName, senderFLS, message string) (*Result, error) {
	audit := store.AuditEntry{Operator: operator, Host: host, Action: "whisper", Subject: "fls=" + targetFLS}

	if !flsRE.MatchString(targetFLS) {
		err := fmt.Errorf("invalid target fls %q (expected hex)", targetFLS)
		audit.Result = "error: " + err.Error()
		_ = p.Store.AppendAudit(audit)
		return nil, err
	}

	ns, pod, err := p.discoverGamePod(ctx, host)
	if err != nil {
		audit.Result = "error: " + err.Error()
		_ = p.Store.AppendAudit(audit)
		return nil, err
	}

	chat := buildWhisperChatJSON(toName, fromName, message, fromName != "")
	payloadB64 := EncodeWhisperEnvelope(chat)
	erlang := BuildErlangWhisper(payloadB64, targetFLS, senderFLS)
	shell := "set -eu; " +
		"export PATH=/opt/rabbitmq/sbin:/opt/erlang/lib/erlang/bin:/bin:/usr/bin:/usr/local/bin:$PATH; " +
		"cat > /tmp/dunemgr-whisper.erl; expr=$(cat /tmp/dunemgr-whisper.erl); " +
		"/opt/rabbitmq/sbin/rabbitmqctl eval \"$expr\"; rm -f /tmp/dunemgr-whisper.erl"
	execRes, err := p.SSH.RunWithStdin(ctx, host, []byte(erlang),
		"kubectl", "exec", "-i", "-n", ns, pod, "--", "sh", "-lc", shell)
	if err != nil {
		audit.Result = "error: " + err.Error()
		_ = p.Store.AppendAudit(audit)
		return nil, fmt.Errorf("kubectl exec rabbitmqctl eval (whisper): %w", err)
	}
	combined := execRes.Stdout
	if strings.TrimSpace(execRes.Stderr) != "" {
		combined += "\n" + execRes.Stderr
	}
	ok := strings.Contains(execRes.Stdout, "publish=ok")
	if ok {
		audit.Result = "ok"
	} else {
		audit.Result = "error: publish not confirmed: " + strings.TrimSpace(combined)
	}
	_ = p.Store.AppendAudit(audit)
	return &Result{OK: ok, RawOutput: combined}, nil
}
