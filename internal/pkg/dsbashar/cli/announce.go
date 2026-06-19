package cli

import (
	"context"
	"time"

	"go.muehmer.eu/dapdsm/pkg/domain/broadcast"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// announceDeps is the injectable seam for tests.
type announceDeps struct {
	announce func(ctx context.Context, delay time.Duration, kind string, action func(context.Context) error) error
}

func defaultAnnounceDeps() announceDeps {
	return announceDeps{announce: realAnnounce}
}

// withAnnounce runs action immediately when delay==0; otherwise it publishes a
// countdown, waits, then runs action (cancelling on error/Ctrl-C) via deps.
func withAnnounce(ctx context.Context, delay time.Duration, kind string,
	action func(context.Context) error, deps announceDeps) error {
	if delay <= 0 {
		return action(ctx)
	}
	if deps.announce == nil {
		deps.announce = realAnnounce
	}
	return deps.announce(ctx, delay, kind, action)
}

// realAnnounce publishes via the LOCAL transport (ds-bashar runs on the node).
func realAnnounce(ctx context.Context, delay time.Duration, kind string, action func(context.Context) error) error {
	st, closeStore, err := openAuditStore()
	if err != nil {
		st = nil // best-effort; mq tolerates a nil Store
	}
	defer closeStore()
	r := &broadcast.Runner{Exec: ssh.LocalExecer{}, Store: st}
	return r.Announce(ctx, broadcast.AnnounceParams{
		Operator: "ds-bashar",
		Host:     "local",
		Kind:     kind,
		Delay:    delay,
	}, action)
}
