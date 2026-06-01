package broadcast

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/mq"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// recordingRunner is a fake ssh.Runner for broadcast tests.
//
// Dispatch logic (Run):
//   - args contain DefaultTokenPath              → tokenStdout (explicit FileToken path)
//   - args contain "get pods" + "rabbitmq"       → mqPodListStdout (newline-sep pod names; must include an mq-game pod)
//   - args contain "get pods" + "battlegroup-deploy" → bgdPodStdout
//   - args contain "get pod" + "secretKeyRef.name" → secretRefNameStdout
//   - args contain "get pod" + "secretKeyRef.key"  → secretRefKeyStdout
//   - args contain "get pod"                      → bgdEnvStdout (literal env value)
//   - args contain "get secret"                   → secretStdout
//   - otherwise                                   → empty success
type recordingRunner struct {
	calls               []call
	tokenStdout         string // used when Runner.TokenPath is set (FileToken path)
	mqPodListStdout     string // newline-separated list of MQ pod names
	bgdPodStdout        string // single bgd-deploy pod name
	bgdEnvStdout        string // literal env value; empty → secretKeyRef path
	secretRefNameStdout string
	secretRefKeyStdout  string
	secretStdout        string // base64-encoded secret data value
	publishStdout       string
}

type call struct {
	name  string
	args  []string
	stdin []byte
}

func (r *recordingRunner) Run(ctx context.Context, name string, args ...string) (ssh.Result, error) {
	r.calls = append(r.calls, call{name: name, args: append([]string(nil), args...)})
	// After the shell-quoting fix ssh.Client passes args as:
	//   ["-o", "BatchMode=yes", "--", host, "<quoted remote cmd>"]
	// The last element is a single token containing all remote-command words
	// individually shell-quoted. Match against it directly.
	joined := strings.Join(args, " ")
	switch {
	case strings.Contains(joined, DefaultTokenPath):
		return ssh.Result{Stdout: r.tokenStdout, ExitCode: 0}, nil
	case strings.Contains(joined, "'get'") && strings.Contains(joined, "'pods'") && strings.Contains(joined, "messagequeue"):
		return ssh.Result{Stdout: r.mqPodListStdout, ExitCode: 0}, nil
	case strings.Contains(joined, "'get'") && strings.Contains(joined, "'pods'") && strings.Contains(joined, "battlegroup-deploy"):
		return ssh.Result{Stdout: r.bgdPodStdout, ExitCode: 0}, nil
	case strings.Contains(joined, "'get'") && strings.Contains(joined, "'pod'") && strings.Contains(joined, "secretKeyRef.name"):
		return ssh.Result{Stdout: r.secretRefNameStdout, ExitCode: 0}, nil
	case strings.Contains(joined, "'get'") && strings.Contains(joined, "'pod'") && strings.Contains(joined, "secretKeyRef.key"):
		return ssh.Result{Stdout: r.secretRefKeyStdout, ExitCode: 0}, nil
	case strings.Contains(joined, "'get'") && strings.Contains(joined, "'pod'"):
		return ssh.Result{Stdout: r.bgdEnvStdout, ExitCode: 0}, nil
	case strings.Contains(joined, "'get'") && strings.Contains(joined, "'secret'"):
		return ssh.Result{Stdout: r.secretStdout, ExitCode: 0}, nil
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

// defaultFakeRunner returns a recordingRunner pre-configured for the common
// happy path: two MQ pods (admin + game) in a BattleGroup namespace using the
// -A ns/pod format, a bgd-deploy pod with a literal env token, and a
// successful publish=ok response.
func defaultFakeRunner(publishOut string) *recordingRunner {
	return &recordingRunner{
		// -A path: "namespace/podname" lines — mirrors what live Funcom clusters emit.
		mqPodListStdout: "funcom-seabass-x/seabass-mq-admin-sts-0\nfuncom-seabass-x/seabass-mq-game-sts-0\n",
		bgdPodStdout:    "dune-bgd-deploy-abc123",
		bgdEnvStdout:    "fake-service-token",
		publishStdout:   publishOut,
	}
}

func TestPublishNoticeRoundtrip(t *testing.T) {
	rr := defaultFakeRunner("publish=ok exchange=heartbeats routing=notifications app_id=fls_backend user_id=fls label=notice\n")
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}

	got, err := r.PublishNotice(context.Background(), "operator", "vm-a", "Title", "Body", 30)
	if err != nil {
		t.Fatalf("PublishNotice: %v", err)
	}
	if !got.OK {
		t.Errorf("OK=false, want true (output=%q)", got.RawOutput)
	}

	// Verify the exec call targets the mq-game pod.
	var execCall *call
	for i := range rr.calls {
		if strings.Contains(strings.Join(rr.calls[i].args, " "), "exec") {
			execCall = &rr.calls[i]
			break
		}
	}
	if execCall == nil {
		t.Fatal("no kubectl exec call recorded")
	}
	if !strings.Contains(strings.Join(execCall.args, " "), "mq-game") {
		t.Errorf("exec call does not target mq-game pod: %v", execCall.args)
	}
	if execCall.stdin == nil || !bytes.Contains(execCall.stdin, []byte("rabbit_queue_type:publish_at_most_once")) {
		t.Errorf("exec stdin missing Erlang: %q", execCall.stdin)
	}

	entries, _ := r.Store.ListAudit(0)
	if len(entries) != 1 || entries[0].Action != "broadcast.notice" || entries[0].Result != "ok" {
		t.Errorf("audit=%+v", entries)
	}
}

func TestPublishShutdownAnnounce(t *testing.T) {
	rr := defaultFakeRunner("publish=ok")
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
	rr := defaultFakeRunner("publish=not_ok")
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	res, _ := r.PublishNotice(context.Background(), "operator", "vm-a", "T", "B", 1)
	if res != nil && res.OK {
		t.Errorf("OK=true for not_ok output, want false")
	}
}

// TestPublishNoticeWithFileToken verifies that setting Runner.TokenPath causes
// FileToken to be used instead of the default chain.
func TestPublishNoticeWithFileToken(t *testing.T) {
	rr := &recordingRunner{
		// -A ns/pod format.
		mqPodListStdout: "funcom-seabass-x/seabass-mq-admin-sts-0\nfuncom-seabass-x/seabass-mq-game-sts-0\n",
		tokenStdout:     "file-token-value\n",
		publishStdout:   "publish=ok",
	}
	r := &Runner{
		SSH:       &ssh.Client{Runner: rr},
		Store:     newTempStore(t),
		TokenPath: mq.DefaultTokenPath,
	}
	got, err := r.PublishNotice(context.Background(), "operator", "vm-a", "Title", "Body", 30)
	if err != nil {
		t.Fatalf("PublishNotice with FileToken: %v", err)
	}
	if !got.OK {
		t.Errorf("OK=false, want true")
	}
	// First call must be the token file read.
	if len(rr.calls) < 1 || !strings.Contains(strings.Join(rr.calls[0].args, " "), DefaultTokenPath) {
		t.Errorf("first call not FileToken read: %v", rr.calls[0].args)
	}
}
