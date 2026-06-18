package broadcast

import (
	"context"
	"fmt"

	"go.muehmer.eu/dapdsm/pkg/domain/mq"
	"go.muehmer.eu/dapdsm/pkg/domain/store"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// Defaults for the MQ pod lookup. These re-export the values from the
// mq package so existing callers that reference broadcast.Default* keep
// working without change.
const (
	DefaultNamespace     = mq.DefaultNamespace
	DefaultMQPodSelector = mq.DefaultMQPodSelector
	DefaultTokenPath     = mq.DefaultTokenPath
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
type Result = mq.Result

// PublishNotice posts a generic banner broadcast.
func (r *Runner) PublishNotice(ctx context.Context, operator, host, title, body string, durationSecs int) (*Result, error) {
	return r.publisher().PublishInner(ctx, operator, host, "broadcast.notice",
		fmt.Sprintf("host=%s title=%q", host, title),
		NoticePayload(title, body, durationSecs),
		"notice",
	)
}

// PublishShutdownAnnounce posts a scheduled-shutdown countdown.
func (r *Runner) PublishShutdownAnnounce(ctx context.Context, operator, host string, a ShutdownAnnounce) (*Result, error) {
	return r.publisher().PublishInner(ctx, operator, host, "broadcast.shutdown",
		fmt.Sprintf("host=%s kind=%s at=%d", host, a.Kind, a.AtUnix),
		ShutdownAnnouncePayload(a),
		"shutdown",
	)
}

// PublishShutdownCancel posts a shutdown-cancel signal.
func (r *Runner) PublishShutdownCancel(ctx context.Context, operator, host string) (*Result, error) {
	return r.publisher().PublishInner(ctx, operator, host, "broadcast.shutdown-cancel",
		fmt.Sprintf("host=%s", host),
		ShutdownCancelPayload(),
		"shutdown-cancel",
	)
}

// publisher constructs a mq.Publisher from the Runner's fields.
// If TokenPath is set, FileToken{Path} is used explicitly; otherwise Token
// is left nil so PublishInner applies the default ChainToken
// (BGEnvToken → LiteralToken(BuiltinToken)).
func (r *Runner) publisher() *mq.Publisher {
	var ts mq.TokenSource
	if r.TokenPath != "" {
		ts = mq.FileToken{Path: r.TokenPath}
	}
	return &mq.Publisher{
		SSH:           r.SSH,
		Store:         r.Store,
		Namespace:     r.Namespace,
		MQPodSelector: r.MQPodSelector,
		Token:         ts,
	}
}
