package broadcast

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

type recordingRunner struct {
	calls         []call
	tokenStdout   string
	mqPodStdout   string
	publishStdout string
}

type call struct {
	name  string
	args  []string
	stdin []byte
}

func (r *recordingRunner) Run(ctx context.Context, name string, args ...string) (ssh.Result, error) {
	r.calls = append(r.calls, call{name: name, args: append([]string(nil), args...)})
	joined := strings.Join(args, " ")
	switch {
	case strings.Contains(joined, "/home/dune/.dune/state/command-auth-token"):
		return ssh.Result{Stdout: r.tokenStdout, ExitCode: 0}, nil
	case strings.Contains(joined, "get pods"):
		return ssh.Result{Stdout: r.mqPodStdout, ExitCode: 0}, nil
	}
	return ssh.Result{ExitCode: 0}, nil
}

func (r *recordingRunner) RunWithStdin(ctx context.Context, stdin []byte, name string, args ...string) (ssh.Result, error) {
	r.calls = append(r.calls, call{name: name, args: append([]string(nil), args...), stdin: append([]byte(nil), stdin...)})
	return ssh.Result{Stdout: r.publishStdout, ExitCode: 0}, nil
}

func newTempStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "dunemgr.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestPublishNoticeRoundtrip(t *testing.T) {
	rr := &recordingRunner{
		tokenStdout:   "TOKEN-XYZ\n",
		mqPodStdout:   "rabbitmq-server-0\n",
		publishStdout: "publish=ok exchange=heartbeats routing=notifications app_id=fls_backend user_id=fls label=notice\n",
	}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}

	got, err := r.PublishNotice(context.Background(), "operator", "vm-a", "Title", "Body", 30)
	if err != nil {
		t.Fatalf("PublishNotice: %v", err)
	}
	if !got.OK {
		t.Errorf("OK=false, want true (output=%q)", got.RawOutput)
	}

	// Sequence: cat token, get pod, exec eval
	if len(rr.calls) != 3 {
		t.Fatalf("calls=%d, want 3 (token, pod, exec)", len(rr.calls))
	}
	if !strings.Contains(strings.Join(rr.calls[0].args, " "), "command-auth-token") {
		t.Errorf("call0 not token read: %v", rr.calls[0].args)
	}
	if !strings.Contains(strings.Join(rr.calls[1].args, " "), "rabbitmq") {
		t.Errorf("call1 not mq pod lookup: %v", rr.calls[1].args)
	}
	if rr.calls[2].stdin == nil || !bytes.Contains(rr.calls[2].stdin, []byte("rabbit_queue_type:publish_at_most_once")) {
		t.Errorf("call2 stdin missing Erlang: %q", rr.calls[2].stdin)
	}

	entries, _ := r.Store.ListAudit(0)
	if len(entries) != 1 || entries[0].Action != "broadcast.notice" || entries[0].Result != "ok" {
		t.Errorf("audit=%+v", entries)
	}
}

func TestPublishShutdownAnnounce(t *testing.T) {
	rr := &recordingRunner{
		tokenStdout:   "T",
		mqPodStdout:   "rabbitmq-server-0",
		publishStdout: "publish=ok",
	}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	if _, err := r.PublishShutdownAnnounce(context.Background(), "operator", "vm-a", ShutdownAnnounce{Kind: "Restart", AtUnix: 1, NowUnix: 0, ShutdownDurationS: 1, BroadcastFrequency: 1, BroadcastDuration: 1}); err != nil {
		t.Fatalf("PublishShutdownAnnounce: %v", err)
	}
	entries, _ := r.Store.ListAudit(0)
	if len(entries) != 1 || entries[0].Action != "broadcast.shutdown" {
		t.Errorf("audit=%+v", entries)
	}
}

func TestPublishMissingPublishOkIsError(t *testing.T) {
	rr := &recordingRunner{
		tokenStdout:   "T",
		mqPodStdout:   "rabbitmq-server-0",
		publishStdout: "publish=not_ok",
	}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	res, _ := r.PublishNotice(context.Background(), "operator", "vm-a", "T", "B", 1)
	if res != nil && res.OK {
		t.Errorf("OK=true for not_ok output, want false")
	}
}
