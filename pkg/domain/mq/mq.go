// Package mq provides the low-level RabbitMQ publish transport used by
// Funcom's dedicated-server stack. It encodes the outer envelope, builds the
// Erlang expression for `rabbitmqctl eval`, and executes the publish over SSH
// + kubectl-exec.
//
// Higher-level packages (e.g. broadcast) build domain-specific payloads and
// delegate to Publisher.PublishInner for the wire path.
package mq

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"go.muehmer.eu/dapdsm/pkg/domain/store"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// HostExecer runs a command (optionally with piped stdin) addressed to a host.
// *ssh.Client satisfies it (commands run over SSH); ssh.LocalExecer satisfies it
// (commands run locally, host ignored). This is the messaging transport seam:
// the RMQ operation is expressed as commands; HostExecer decides where they run.
type HostExecer interface {
	Run(ctx context.Context, host, name string, args ...string) (ssh.Result, error)
	RunWithStdin(ctx context.Context, host string, stdin []byte, name string, args ...string) (ssh.Result, error)
}

// Defaults for the MQ pod lookup and token path. Override via Publisher
// fields; zero values fall back to these constants.
//
// DefaultNamespace is kept for backward compatibility (e.g. BGEnvToken) but
// is no longer used as a default in PublishInner — that path now discovers the
// namespace via -A when Publisher.Namespace is not set.
const (
	DefaultNamespace = "dune"
	// DefaultMQPodSelector targets the game RabbitMQ broker by Funcom's labels
	// (verified live: the heartbeats/chat.whispers exchanges live on mq-game).
	DefaultMQPodSelector = "role=igw-message-queue,messagequeue=game"
	DefaultTokenPath     = "/home/dune/.dune/state/command-auth-token"

	// DefaultBGDPodSelector is the label selector for the BattleGroup director pod
	// (verified live: it carries the FuncomLiveServices__ServiceAuthToken env).
	DefaultBGDPodSelector = "role=igw-battlegroup-director"

	// DefaultBGDEnvVar is the env-var name that carries the FLS service auth token
	// in the BattleGroup deploy pod.
	DefaultBGDEnvVar = "FuncomLiveServices__ServiceAuthToken"

	// BuiltinToken is the ddsm built-in fallback auth token. Used as the last
	// resort in the default ChainToken when no other source provides a value.
	BuiltinToken = "Nu6VmPWUMvdPMeB7qErr"
)

// Constants matching the Funcom-side RabbitMQ topology.
const (
	exchange   = "heartbeats"
	routingKey = "notifications"
	userID     = "fls"
	appID      = "fls_backend"
)

var safeLabelRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

// Result captures the outcome of one publish.
type Result struct {
	OK        bool
	RawOutput string
}

// TokenSource provides the auth token needed to authenticate with the
// Funcom FLS backend. Implementations may read the token from disk via
// SSH, fetch it from an API, or return a static value for tests.
type TokenSource interface {
	Token(ctx context.Context, exec HostExecer, host string) (string, error)
}

// FileToken reads the auth token from a file on the remote host via
// SSH `cat <Path>`. The token is trimmed of surrounding whitespace; an
// empty result after trimming is treated as an error.
type FileToken struct {
	Path string
}

// Token implements TokenSource by running `cat <Path>` on the remote host.
func (f FileToken) Token(ctx context.Context, exec HostExecer, host string) (string, error) {
	res, err := exec.Run(ctx, host, "cat", f.Path)
	if err != nil || res.ExitCode != 0 {
		return "", fmt.Errorf("read auth token from %s: %w", f.Path, err)
	}
	token := strings.TrimSpace(res.Stdout)
	if token == "" {
		return "", fmt.Errorf("auth token at %s is empty", f.Path)
	}
	return token, nil
}

// LiteralToken returns a fixed token value. Use mq.BuiltinToken as the
// value to get the ddsm built-in fallback.
type LiteralToken struct {
	Value string
}

// Token implements TokenSource by returning the fixed Value.
// Returns an error if Value is empty.
func (t LiteralToken) Token(_ context.Context, _ HostExecer, _ string) (string, error) {
	if t.Value == "" {
		return "", fmt.Errorf("LiteralToken: value is empty")
	}
	return t.Value, nil
}

// BGEnvToken reads the auth token from the BattleGroup deploy pod's environment.
//
// EnvVar defaults to DefaultBGDEnvVar; BGSelector defaults to DefaultBGDPodSelector.
//
// Resolution order:
//  1. Look up the bgd-deploy pod via BGSelector.
//  2. Try the literal env value via jsonpath .spec.containers[*].env[?(@.name=="<EnvVar>")].value.
//  3. If empty, resolve via secretKeyRef: read the secret name and key, then
//     kubectl-get-secret and base64-decode the data field.
//
// The token is trimmed of surrounding whitespace. An error is returned if both
// attempts yield an empty result. The token value is never logged.
type BGEnvToken struct {
	// EnvVar is the env-var name to read. Defaults to DefaultBGDEnvVar.
	EnvVar string
	// BGSelector is the label selector for the bgd-deploy pod.
	// Defaults to DefaultBGDPodSelector.
	BGSelector string
}

// envVar returns the effective env-var name.
func (t BGEnvToken) envVar() string {
	if t.EnvVar != "" {
		return t.EnvVar
	}
	return DefaultBGDEnvVar
}

// bgSelector returns the effective pod selector.
func (t BGEnvToken) bgSelector() string {
	if t.BGSelector != "" {
		return t.BGSelector
	}
	return DefaultBGDPodSelector
}

// Token implements TokenSource by reading the env-var from the bgd-deploy pod.
func (t BGEnvToken) Token(ctx context.Context, exec HostExecer, host string) (string, error) {
	ns := DefaultNamespace
	sel := t.bgSelector()
	envVar := t.envVar()

	// 1. Find the bgd-deploy pod.
	podRes, err := exec.Run(ctx, host, "kubectl", "get", "pods",
		"-n", ns, "-l", sel,
		"-o", "jsonpath={.items[0].metadata.name}")
	if err != nil || podRes.ExitCode != 0 {
		return "", fmt.Errorf("BGEnvToken: find bgd-deploy pod in %s (selector %s): %w", ns, sel, err)
	}
	pod := strings.TrimSpace(podRes.Stdout)
	if pod == "" {
		return "", fmt.Errorf("BGEnvToken: no bgd-deploy pod found in %s for selector %s", ns, sel)
	}

	// 2. Try literal env value.
	literalExpr := fmt.Sprintf(
		`{range .spec.containers[*].env[?(@.name=="%s")]}{.value}{end}`,
		envVar,
	)
	litRes, err := exec.Run(ctx, host, "kubectl", "get", "pod", pod,
		"-n", ns, "-o", "jsonpath="+literalExpr)
	if err == nil && litRes.ExitCode == 0 {
		if tok := strings.TrimSpace(litRes.Stdout); tok != "" {
			return tok, nil
		}
	}

	// 3. Resolve secretKeyRef.
	secretNameExpr := fmt.Sprintf(
		`{range .spec.containers[*].env[?(@.name=="%s")]}{.valueFrom.secretKeyRef.name}{end}`,
		envVar,
	)
	secretKeyExpr := fmt.Sprintf(
		`{range .spec.containers[*].env[?(@.name=="%s")]}{.valueFrom.secretKeyRef.key}{end}`,
		envVar,
	)
	secretNameRes, err := exec.Run(ctx, host, "kubectl", "get", "pod", pod,
		"-n", ns, "-o", "jsonpath="+secretNameExpr)
	if err != nil || secretNameRes.ExitCode != 0 {
		return "", fmt.Errorf("BGEnvToken: read secretKeyRef.name from pod %s: %w", pod, err)
	}
	secretKeyRes, err := exec.Run(ctx, host, "kubectl", "get", "pod", pod,
		"-n", ns, "-o", "jsonpath="+secretKeyExpr)
	if err != nil || secretKeyRes.ExitCode != 0 {
		return "", fmt.Errorf("BGEnvToken: read secretKeyRef.key from pod %s: %w", pod, err)
	}
	secretName := strings.TrimSpace(secretNameRes.Stdout)
	secretKey := strings.TrimSpace(secretKeyRes.Stdout)
	if secretName == "" || secretKey == "" {
		return "", fmt.Errorf("BGEnvToken: env var %s in pod %s has no literal value and no secretKeyRef", envVar, pod)
	}

	// Fetch the secret and base64-decode the key value.
	secretDataExpr := fmt.Sprintf("{.data.%s}", secretKey)
	secRes, err := exec.Run(ctx, host, "kubectl", "get", "secret", secretName,
		"-n", ns, "-o", "jsonpath="+secretDataExpr)
	if err != nil || secRes.ExitCode != 0 {
		return "", fmt.Errorf("BGEnvToken: read secret %s key %s: %w", secretName, secretKey, err)
	}
	encoded := strings.TrimSpace(secRes.Stdout)
	if encoded == "" {
		return "", fmt.Errorf("BGEnvToken: secret %s key %s is empty", secretName, secretKey)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("BGEnvToken: base64-decode secret %s key %s: %w", secretName, secretKey, err)
	}
	tok := strings.TrimSpace(string(decoded))
	if tok == "" {
		return "", fmt.Errorf("BGEnvToken: decoded token from secret %s key %s is empty", secretName, secretKey)
	}
	return tok, nil
}

// ChainToken tries each source in order, returning the first non-empty token.
// If all sources fail, the last error is returned.
type ChainToken struct {
	Sources []TokenSource
}

// Token implements TokenSource by trying each source in order.
func (t ChainToken) Token(ctx context.Context, exec HostExecer, host string) (string, error) {
	var lastErr error
	for _, src := range t.Sources {
		tok, err := src.Token(ctx, exec, host)
		if err == nil && tok != "" {
			return tok, nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		return "", fmt.Errorf("ChainToken: all sources failed, last error: %w", lastErr)
	}
	return "", fmt.Errorf("ChainToken: all sources returned empty tokens")
}

// defaultTokenChain returns the default ChainToken used when Publisher.Token is nil:
// BGEnvToken → LiteralToken(BuiltinToken).
func defaultTokenChain() ChainToken {
	return ChainToken{Sources: []TokenSource{
		BGEnvToken{},
		LiteralToken{Value: BuiltinToken},
	}}
}

// pickGamePod returns the first pod name in names that contains "mq-game".
// Returns ("", false) if no match is found.
//
// Deprecated: use pickGameNsPod when pod names include a namespace prefix.
func pickGamePod(names []string) (string, bool) {
	for _, n := range names {
		if strings.Contains(n, "mq-game") {
			return n, true
		}
	}
	return "", false
}

// pickGameNsPod parses lines of the form "namespace/podname" and returns the
// namespace and pod name of the first entry whose pod name contains "mq-game".
// Returns ("", "", false) if no matching line is found.
func pickGameNsPod(lines []string) (ns, pod string, ok bool) {
	for _, line := range lines {
		slash := strings.IndexByte(line, '/')
		if slash < 0 {
			continue
		}
		podName := line[slash+1:]
		if strings.Contains(podName, "mq-game") {
			return line[:slash], podName, true
		}
	}
	return "", "", false
}

// Publisher publishes a single encoded message to the Funcom RabbitMQ
// exchange via SSH + kubectl-exec. Zero values for Namespace,
// MQPodSelector, and Token fall back to the Default* constants /
// defaultTokenChain().
type Publisher struct {
	Exec          HostExecer
	Store         *store.Store
	Namespace     string
	MQPodSelector string
	Token         TokenSource
}

// appendAudit records the entry if a Store is configured; a nil Store (e.g. a
// CLI without a data dir) silently skips auditing.
func (p *Publisher) appendAudit(e store.AuditEntry) {
	if p.Store != nil {
		_ = p.Store.AppendAudit(e)
	}
}

// discoverGamePod resolves the (namespace, pod) of the mq-game broker,
// honoring p.Namespace / p.MQPodSelector. Shared by PublishInner and
// PublishWhisper; returns a plain error without touching the audit log.
//
// Discovery strategy:
//   - When Publisher.Namespace is set, query is scoped to that namespace;
//     pod names are returned without a namespace prefix.
//   - When Publisher.Namespace == "" (the default), query uses -A (all
//     namespaces) and emits "namespace/podname" lines so both are resolved
//     in one call. This is required because MQ pods live in the BattleGroup
//     namespace (e.g. funcom-seabass-…), not "dune".
func (p *Publisher) discoverGamePod(ctx context.Context, host string) (ns, pod string, err error) {
	sel := p.MQPodSelector
	if sel == "" {
		sel = DefaultMQPodSelector
	}

	if p.Namespace != "" {
		// Explicit namespace override: scope to it, plain pod names.
		podListRes, lerr := p.Exec.Run(ctx, host, "kubectl", "get", "pods",
			"-n", p.Namespace, "-l", sel,
			"-o", `jsonpath={range .items[*]}{.metadata.name}{"\n"}{end}`)
		if lerr != nil || podListRes.ExitCode != 0 {
			return "", "", fmt.Errorf("list mq pods in %s (selector %s): %w", p.Namespace, sel, lerr)
		}
		var podNames []string
		for _, line := range strings.Split(podListRes.Stdout, "\n") {
			if name := strings.TrimSpace(line); name != "" {
				podNames = append(podNames, name)
			}
		}
		picked, ok := pickGamePod(podNames)
		if !ok {
			return "", "", fmt.Errorf("no mq-game broker pod found in %s for selector %s", p.Namespace, sel)
		}
		return p.Namespace, picked, nil
	}

	// Auto-discover: query all namespaces, emit namespace/podname per line.
	podListRes, lerr := p.Exec.Run(ctx, host, "kubectl", "get", "pods",
		"-A", "-l", sel,
		"-o", `jsonpath={range .items[*]}{.metadata.namespace}{"/"}{.metadata.name}{"\n"}{end}`)
	if lerr != nil || podListRes.ExitCode != 0 {
		return "", "", fmt.Errorf("list mq pods across all namespaces (selector %s): %w", sel, lerr)
	}
	var lines []string
	for _, line := range strings.Split(podListRes.Stdout, "\n") {
		if l := strings.TrimSpace(line); l != "" {
			lines = append(lines, l)
		}
	}
	ns, pod, ok := pickGameNsPod(lines)
	if !ok {
		return "", "", fmt.Errorf("no mq-game broker pod found across all namespaces for selector %s", sel)
	}
	return ns, pod, nil
}

// PublishInner encodes innerJSON inside the Funcom envelope, locates the
// mq-game RabbitMQ pod, and pipes the Erlang expression to `rabbitmqctl eval`.
// The audit entry is written to the Store regardless of outcome.
//
// Pod/namespace discovery is delegated to discoverGamePod (shared with
// PublishWhisper); discovery errors are written to the audit log here.
//
// Parameters:
//
//	operator — identity of the acting operator (for audit).
//	host     — SSH alias of the target VM.
//	action   — audit action label (e.g. "broadcast.notice").
//	subject  — free-form audit subject.
//	innerJSON — already-serialized inner ServerCommand payload.
//	label     — short identifier for the server-side log line; sanitized
//	            against an allowlist to prevent Erlang-string injection.
func (p *Publisher) PublishInner(ctx context.Context, operator, host, action, subject, innerJSON, label string) (*Result, error) {
	ts := p.Token
	if ts == nil {
		ts = defaultTokenChain()
	}

	audit := store.AuditEntry{
		Operator: operator,
		Host:     host,
		Action:   action,
		Subject:  subject,
	}

	// 1. Read AuthToken
	token, err := ts.Token(ctx, p.Exec, host)
	if err != nil {
		audit.Result = "error: " + err.Error()
		p.appendAudit(audit)
		return nil, err
	}

	// 2. Discover the mq-game broker pod and its namespace.
	ns, pod, err := p.discoverGamePod(ctx, host)
	if err != nil {
		audit.Result = "error: " + err.Error()
		p.appendAudit(audit)
		return nil, err
	}

	// 3. Build payload + Erlang and exec using the discovered (or explicit) namespace.
	payloadB64 := EncodeEnvelope([]byte(innerJSON), token)
	erlang := BuildErlangPublish(payloadB64, label)
	shell := "set -eu; " +
		"export PATH=/opt/rabbitmq/sbin:/opt/erlang/lib/erlang/bin:/bin:/usr/bin:/usr/local/bin:$PATH; " +
		"cat > /tmp/dunemgr-mq-publish.erl; " +
		"expr=$(cat /tmp/dunemgr-mq-publish.erl); " +
		"/opt/rabbitmq/sbin/rabbitmqctl eval \"$expr\"; " +
		"rm -f /tmp/dunemgr-mq-publish.erl"
	execRes, err := p.Exec.RunWithStdin(ctx, host, []byte(erlang),
		"kubectl", "exec", "-i", "-n", ns, pod, "--", "sh", "-lc", shell)
	if err != nil {
		audit.Result = "error: " + err.Error()
		p.appendAudit(audit)
		return nil, fmt.Errorf("kubectl exec rabbitmqctl eval: %w", err)
	}
	combined := execRes.Stdout
	if strings.TrimSpace(execRes.Stderr) != "" {
		combined = combined + "\n" + execRes.Stderr
	}
	ok := strings.Contains(execRes.Stdout, "publish=ok")
	if ok {
		audit.Result = "ok"
	} else {
		audit.Result = "error: publish not confirmed: " + strings.TrimSpace(combined)
	}
	p.appendAudit(audit)
	return &Result{OK: ok, RawOutput: combined}, nil
}

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
