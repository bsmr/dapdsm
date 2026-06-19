package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"go.muehmer.eu/dapdsm/pkg/domain/broadcast"
	"go.muehmer.eu/dapdsm/pkg/domain/mq"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// DefaultNoticeDuration is how long a banner stays up (seconds) when --duration
// is not given.
const DefaultNoticeDuration = 30

// broadcastTransport selects where the publish runs.
type broadcastTransport struct {
	local bool
	exec  mq.HostExecer
}

// broadcastDeps is the injectable seam for tests.
type broadcastDeps struct {
	publish func(ctx context.Context, tr broadcastTransport, host, title, body string, durationSecs int, stderr io.Writer) error
}

func defaultBroadcastDeps() broadcastDeps {
	return broadcastDeps{publish: publishNotice}
}

func broadcastCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return runBroadcast(ctx, args, stdout, stderr, defaultBroadcastDeps())
}

func runBroadcast(ctx context.Context, args []string, stdout, stderr io.Writer, deps broadcastDeps) error {
	fs := flag.NewFlagSet("broadcast", flag.ContinueOnError)
	fs.SetOutput(stderr)
	title := fs.String("title", "", "Banner title")
	dur := fs.Int("duration", DefaultNoticeDuration, "Banner duration (seconds)")
	sshAlias := fs.String("ssh", "", "Publish over SSH to this host alias (default: local kubectl)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("broadcast: %w: %w", ErrUsage, err)
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "broadcast: missing message text")
		return ErrUsage
	}
	body := strings.Join(fs.Args(), " ")

	tr := broadcastTransport{local: *sshAlias == ""}
	host := "local"
	if tr.local {
		tr.exec = ssh.LocalExecer{}
	} else {
		tr.exec = ssh.NewClient()
		host = *sshAlias
	}
	if deps.publish == nil {
		deps.publish = publishNotice
	}
	if err := deps.publish(ctx, tr, host, *title, body, *dur, stderr); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "broadcast: published")
	return nil
}

// publishNotice is the real publish path: open the audit store best-effort,
// build a broadcast.Runner on the chosen transport, send the banner.
func publishNotice(ctx context.Context, tr broadcastTransport, host, title, body string, durationSecs int, stderr io.Writer) error {
	st, closeStore, err := openAuditStore()
	if err != nil {
		fmt.Fprintf(stderr, "broadcast: audit store unavailable (%v); proceeding without audit\n", err)
		st = nil // mq.Publisher tolerates a nil Store
	}
	defer closeStore()
	r := &broadcast.Runner{Exec: tr.exec, Store: st}
	_, err = r.PublishNotice(ctx, "ds-bashar", host, title, body, durationSecs)
	return err
}
