package broadcast

import (
	"context"
	"fmt"
	"strings"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// Defaults for the MQ pod lookup. Override via Runner fields if a
// future operator wants a different namespace / label.
const (
	DefaultNamespace     = "dune"
	DefaultMQPodSelector = "app.kubernetes.io/name=rabbitmq"
	DefaultTokenPath     = "/home/dune/.dune/state/command-auth-token"
)

// Runner publishes broadcasts against a host. The wire path is:
//  1. SSH `cat <TokenPath>` to read the AuthToken
//  2. SSH `kubectl get pods -n <Namespace> -l <Selector> -o ...` to find the MQ pod
//  3. SSH `kubectl exec -i -n <Namespace> <mq-pod> -- sh -lc <shell>` piping Erlang
type Runner struct {
	SSH   *ssh.Client
	Store *store.Store
	// Optional overrides. Zero values map to the Default* constants.
	Namespace     string
	MQPodSelector string
	TokenPath     string
}

// Result captures the outcome of one publish.
type Result struct {
	OK        bool
	RawOutput string
}

// PublishNotice posts a generic banner broadcast.
func (r *Runner) PublishNotice(ctx context.Context, operator, host, title, body string, durationSecs int) (*Result, error) {
	return r.publish(ctx, operator, host, "broadcast.notice",
		fmt.Sprintf("host=%s title=%q", host, title),
		NoticePayload(title, body, durationSecs),
		"notice",
	)
}

// PublishShutdownAnnounce posts a scheduled-shutdown countdown.
func (r *Runner) PublishShutdownAnnounce(ctx context.Context, operator, host string, a ShutdownAnnounce) (*Result, error) {
	return r.publish(ctx, operator, host, "broadcast.shutdown",
		fmt.Sprintf("host=%s kind=%s at=%d", host, a.Kind, a.AtUnix),
		ShutdownAnnouncePayload(a),
		"shutdown",
	)
}

// PublishShutdownCancel posts a shutdown-cancel signal.
func (r *Runner) PublishShutdownCancel(ctx context.Context, operator, host string) (*Result, error) {
	return r.publish(ctx, operator, host, "broadcast.shutdown-cancel",
		fmt.Sprintf("host=%s", host),
		ShutdownCancelPayload(),
		"shutdown-cancel",
	)
}

func (r *Runner) publish(ctx context.Context, operator, host, action, subject, innerJSON, label string) (*Result, error) {
	ns := r.Namespace
	if ns == "" {
		ns = DefaultNamespace
	}
	sel := r.MQPodSelector
	if sel == "" {
		sel = DefaultMQPodSelector
	}
	tokenPath := r.TokenPath
	if tokenPath == "" {
		tokenPath = DefaultTokenPath
	}

	audit := store.AuditEntry{
		Operator: operator,
		Host:     host,
		Action:   action,
		Subject:  subject,
	}

	// 1. Read AuthToken
	tokRes, err := r.SSH.Run(ctx, host, "cat", tokenPath)
	if err != nil || tokRes.ExitCode != 0 {
		audit.Result = fmt.Sprintf("error: read token: %v exit=%d", err, tokRes.ExitCode)
		_ = r.Store.AppendAudit(audit)
		return nil, fmt.Errorf("read auth token from %s: %w", tokenPath, err)
	}
	token := strings.TrimSpace(tokRes.Stdout)
	if token == "" {
		audit.Result = "error: empty token"
		_ = r.Store.AppendAudit(audit)
		return nil, fmt.Errorf("auth token at %s is empty", tokenPath)
	}

	// 2. Find MQ pod
	podRes, err := r.SSH.Run(ctx, host, "kubectl", "get", "pods", "-n", ns, "-l", sel,
		"-o", "jsonpath={.items[0].metadata.name}")
	if err != nil || podRes.ExitCode != 0 {
		audit.Result = fmt.Sprintf("error: find mq pod: %v exit=%d", err, podRes.ExitCode)
		_ = r.Store.AppendAudit(audit)
		return nil, fmt.Errorf("find mq pod in %s/%s: %w", ns, sel, err)
	}
	pod := strings.TrimSpace(podRes.Stdout)
	if pod == "" {
		audit.Result = "error: no mq pod matched selector"
		_ = r.Store.AppendAudit(audit)
		return nil, fmt.Errorf("no mq pod matched selector %q in namespace %q", sel, ns)
	}

	// 3. Build payload + Erlang and exec
	payloadB64 := EncodeEnvelope([]byte(innerJSON), token)
	erlang := BuildErlangPublish(payloadB64, label)
	shell := "set -eu; " +
		"export PATH=/opt/rabbitmq/sbin:/opt/erlang/lib/erlang/bin:/bin:/usr/bin:/usr/local/bin:$PATH; " +
		"cat > /tmp/dunemgr-mq-publish.erl; " +
		"expr=$(cat /tmp/dunemgr-mq-publish.erl); " +
		"/opt/rabbitmq/sbin/rabbitmqctl eval \"$expr\"; " +
		"rm -f /tmp/dunemgr-mq-publish.erl"
	execRes, err := r.SSH.RunWithStdin(ctx, host, []byte(erlang),
		"kubectl", "exec", "-i", "-n", ns, pod, "--", "sh", "-lc", shell)
	if err != nil {
		audit.Result = "error: " + err.Error()
		_ = r.Store.AppendAudit(audit)
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
	_ = r.Store.AppendAudit(audit)
	return &Result{OK: ok, RawOutput: combined}, nil
}
