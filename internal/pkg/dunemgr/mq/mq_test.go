package mq

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// recordingRunner is a fake ssh.Runner that records calls and returns
// pre-configured responses by inspecting argument patterns.
//
// Dispatch logic (Run):
//   - args contain DefaultTokenPath             → tokenStdout
//   - args contain "get pods" + "rabbitmq"      → mqPodListStdout
//     When the query uses -A (all-ns), mqPodListStdout should contain
//     "namespace/podname" lines (e.g. "funcom-seabass-x/…-mq-game-sts-0\n").
//     When scoped to -n <ns>, plain pod-name lines are expected.
//   - args contain "get pods" + "battlegroup-director" → bgdPodStdout (bgd-deploy pod name)
//   - args contain "get pod" + ".value"         → bgdEnvStdout (literal env value from jsonpath)
//   - args contain "get pod" + "secretKeyRef.name" → secretRefNameStdout
//   - args contain "get pod" + "secretKeyRef.key"  → secretRefKeyStdout
//   - args contain "get secret"                 → secretStdout (base64-encoded secret value)
//   - otherwise                                 → empty success
type recordingRunner struct {
	calls               []call
	tokenStdout         string
	mqPodListStdout     string // pod list output (ns/pod lines for -A path, plain names for -n path)
	bgdPodStdout        string // single bgd-deploy pod name
	bgdEnvStdout        string // literal env value returned by jsonpath; empty → secretKeyRef path
	secretRefNameStdout string // secret name returned by secretKeyRef.name lookup
	secretRefKeyStdout  string // secret key returned by secretKeyRef.key lookup
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
	// individually shell-quoted. Match against it directly so patterns are
	// independent of the surrounding ssh-flag prefix.
	joined := strings.Join(args, " ")
	switch {
	case strings.Contains(joined, DefaultTokenPath):
		return ssh.Result{Stdout: r.tokenStdout, ExitCode: 0}, nil
	case strings.Contains(joined, "'get'") && strings.Contains(joined, "'pods'") && strings.Contains(joined, "messagequeue"):
		return ssh.Result{Stdout: r.mqPodListStdout, ExitCode: 0}, nil
	case strings.Contains(joined, "'get'") && strings.Contains(joined, "'pods'") && strings.Contains(joined, "battlegroup-director"):
		return ssh.Result{Stdout: r.bgdPodStdout, ExitCode: 0}, nil
	case strings.Contains(joined, "'get'") && strings.Contains(joined, "'pod'") && strings.Contains(joined, "secretKeyRef.name"):
		return ssh.Result{Stdout: r.secretRefNameStdout, ExitCode: 0}, nil
	case strings.Contains(joined, "'get'") && strings.Contains(joined, "'pod'") && strings.Contains(joined, "secretKeyRef.key"):
		return ssh.Result{Stdout: r.secretRefKeyStdout, ExitCode: 0}, nil
	case strings.Contains(joined, "'get'") && strings.Contains(joined, "'pod'"):
		// literal env value lookup
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

// --- pickGamePod ---

// TestPickGamePod_Match verifies that the first mq-game pod is returned.
func TestPickGamePod_Match(t *testing.T) {
	names := []string{
		"dune-mq-admin-sts-0",
		"dune-mq-game-sts-0",
	}
	got, ok := pickGamePod(names)
	if !ok {
		t.Fatal("want ok=true, got false")
	}
	if got != "dune-mq-game-sts-0" {
		t.Errorf("got %q, want %q", got, "dune-mq-game-sts-0")
	}
}

// TestPickGamePod_NoMatch verifies that ("",false) is returned when no mq-game pod exists.
func TestPickGamePod_NoMatch(t *testing.T) {
	names := []string{"dune-rabbitmq-0"}
	_, ok := pickGamePod(names)
	if ok {
		t.Error("want ok=false for no mq-game pod, got true")
	}
}

// TestPickGamePod_Empty verifies that an empty slice returns false.
func TestPickGamePod_Empty(t *testing.T) {
	_, ok := pickGamePod(nil)
	if ok {
		t.Error("want ok=false for empty slice, got true")
	}
}

// TestPickGamePod_FirstMatch verifies that the first matching pod wins when there are multiples.
func TestPickGamePod_FirstMatch(t *testing.T) {
	names := []string{"dune-mq-game-sts-0", "dune-mq-game-sts-1", "dune-mq-admin-sts-0"}
	got, ok := pickGamePod(names)
	if !ok {
		t.Fatal("want ok=true")
	}
	if got != "dune-mq-game-sts-0" {
		t.Errorf("got %q, want first match %q", got, "dune-mq-game-sts-0")
	}
}

// --- pickGameNsPod ---

// TestPickGameNsPod_Match verifies that the mq-game entry is parsed and both
// namespace and pod name are returned correctly.
func TestPickGameNsPod_Match(t *testing.T) {
	lines := []string{
		"funcom-seabass-x/seabass-mq-admin-sts-0",
		"funcom-seabass-x/seabass-mq-game-sts-0",
	}
	ns, pod, ok := pickGameNsPod(lines)
	if !ok {
		t.Fatal("want ok=true, got false")
	}
	if ns != "funcom-seabass-x" {
		t.Errorf("ns=%q, want %q", ns, "funcom-seabass-x")
	}
	if pod != "seabass-mq-game-sts-0" {
		t.Errorf("pod=%q, want %q", pod, "seabass-mq-game-sts-0")
	}
}

// TestPickGameNsPod_NoMatch verifies ("","",false) when no mq-game entry is present.
func TestPickGameNsPod_NoMatch(t *testing.T) {
	lines := []string{"funcom-seabass-x/seabass-mq-admin-sts-0"}
	_, _, ok := pickGameNsPod(lines)
	if ok {
		t.Error("want ok=false for no mq-game entry, got true")
	}
}

// TestPickGameNsPod_Empty verifies ("","",false) for an empty slice.
func TestPickGameNsPod_Empty(t *testing.T) {
	_, _, ok := pickGameNsPod(nil)
	if ok {
		t.Error("want ok=false for empty slice, got true")
	}
}

// TestPickGameNsPod_SkipsNoSlash verifies that lines without a slash are skipped.
func TestPickGameNsPod_SkipsNoSlash(t *testing.T) {
	lines := []string{"no-slash-mq-game", "funcom-seabass-x/seabass-mq-game-sts-0"}
	ns, pod, ok := pickGameNsPod(lines)
	if !ok {
		t.Fatal("want ok=true for second entry, got false")
	}
	if ns != "funcom-seabass-x" || pod != "seabass-mq-game-sts-0" {
		t.Errorf("ns=%q pod=%q, want funcom-seabass-x / seabass-mq-game-sts-0", ns, pod)
	}
}

// TestPickGameNsPod_FirstMatch verifies that the first mq-game entry wins.
func TestPickGameNsPod_FirstMatch(t *testing.T) {
	lines := []string{
		"funcom-seabass-x/seabass-mq-game-sts-0",
		"funcom-seabass-x/seabass-mq-game-sts-1",
	}
	ns, pod, ok := pickGameNsPod(lines)
	if !ok {
		t.Fatal("want ok=true")
	}
	if ns != "funcom-seabass-x" || pod != "seabass-mq-game-sts-0" {
		t.Errorf("got ns=%q pod=%q, want first match", ns, pod)
	}
}

// --- PublishInner with game-broker pick ---

// TestPublishInnerRoundtrip covers the full happy path with mq-game broker.
// The pod list uses the -A ns/pod format; the game pod must be selected and
// its discovered namespace (funcom-seabass-x, NOT "dune") must be used in exec.
func TestPublishInnerRoundtrip(t *testing.T) {
	rr := &recordingRunner{
		// -A path: "namespace/podname" lines.
		mqPodListStdout: "funcom-seabass-x/seabass-mq-admin-sts-0\nfuncom-seabass-x/seabass-mq-game-sts-0\n",
		bgdPodStdout:    "dune-bgd-deploy-abc123",
		bgdEnvStdout:    "fake-service-token",
		publishStdout:   "publish=ok exchange=heartbeats routing=notifications app_id=fls_backend user_id=fls label=notice\n",
	}
	pub := &Publisher{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}

	res, err := pub.PublishInner(context.Background(), "operator", "vm-a",
		"broadcast.notice", "host=vm-a title=\"Title\"",
		`{"ServerCommand":"ServiceBroadcast"}`, "notice")
	if err != nil {
		t.Fatalf("PublishInner: %v", err)
	}
	if !res.OK {
		t.Errorf("OK=false, want true (output=%q)", res.RawOutput)
	}

	// Verify the exec call targets the mq-game pod.
	var execCall *call
	for i := range rr.calls {
		joined := strings.Join(rr.calls[i].args, " ")
		if strings.Contains(joined, "exec") {
			execCall = &rr.calls[i]
			break
		}
	}
	if execCall == nil {
		t.Fatal("no kubectl exec call recorded")
	}
	execArgv := strings.Join(execCall.args, " ")
	if !strings.Contains(execArgv, "mq-game") {
		t.Errorf("exec call does not target mq-game pod: %v", execCall.args)
	}
	// The discovered namespace (funcom-seabass-x) must be used — not "dune".
	if !strings.Contains(execArgv, "funcom-seabass-x") {
		t.Errorf("exec does not use discovered namespace funcom-seabass-x: %v", execCall.args)
	}
	// Check that "dune" does not appear as the -n flag value (the shell payload
	// may reference "/home/dune/..." paths, so bare substring checks are avoided).
	if strings.Contains(execArgv, `'-n' 'dune'`) || strings.Contains(execArgv, `"-n" "dune"`) {
		t.Errorf("exec uses hardcoded 'dune' namespace instead of discovered one: %v", execCall.args)
	}
	if execCall.stdin == nil || !bytes.Contains(execCall.stdin, []byte("rabbit_queue_type:publish_at_most_once")) {
		t.Errorf("exec stdin missing Erlang: %q", execCall.stdin)
	}

	entries, _ := pub.Store.ListAudit(0)
	if len(entries) != 1 || entries[0].Action != "broadcast.notice" || entries[0].Result != "ok" {
		t.Errorf("audit=%+v", entries)
	}
}

// TestPublishInner_NoGamePod verifies an error when no mq-game pod is found.
func TestPublishInner_NoGamePod(t *testing.T) {
	rr := &recordingRunner{
		// -A path: only admin pod, no mq-game.
		mqPodListStdout: "funcom-seabass-x/seabass-mq-admin-sts-0\n",
		bgdPodStdout:    "dune-bgd-deploy-abc123",
		bgdEnvStdout:    "fake-service-token",
		publishStdout:   "",
	}
	pub := &Publisher{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	_, err := pub.PublishInner(context.Background(), "operator", "vm-a",
		"broadcast.notice", "host=vm-a", `{}`, "notice")
	if err == nil {
		t.Error("expected error for missing mq-game pod, got nil")
	}
	if !strings.Contains(err.Error(), "no mq-game broker pod") {
		t.Errorf("error does not mention mq-game: %v", err)
	}
}

// TestPublishInnerMissingPublishOk verifies that the absence of
// "publish=ok" in rabbitmqctl output sets Result.OK=false.
func TestPublishInnerMissingPublishOk(t *testing.T) {
	rr := &recordingRunner{
		// -A path with a single mq-game entry.
		mqPodListStdout: "funcom-seabass-x/seabass-mq-game-sts-0\n",
		bgdPodStdout:    "dune-bgd-deploy-abc123",
		bgdEnvStdout:    "fake-service-token",
		publishStdout:   "publish=not_ok",
	}
	pub := &Publisher{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	res, _ := pub.PublishInner(context.Background(), "operator", "vm-a",
		"broadcast.notice", "host=vm-a", `{}`, "notice")
	if res != nil && res.OK {
		t.Errorf("OK=true for not_ok output, want false")
	}
}

// TestPublishInner_DiscoveredNamespaceNotDune asserts that when Publisher.Namespace
// is empty the exec uses the namespace discovered from the pod list, not "dune".
func TestPublishInner_DiscoveredNamespaceNotDune(t *testing.T) {
	const wantNS = "funcom-seabass-prod"
	rr := &recordingRunner{
		mqPodListStdout: wantNS + "/prod-mq-game-sts-0\n",
		bgdPodStdout:    "dune-bgd-deploy-abc123",
		bgdEnvStdout:    "fake-token",
		publishStdout:   "publish=ok",
	}
	pub := &Publisher{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	if _, err := pub.PublishInner(context.Background(), "op", "vm-a",
		"test.action", "subj", `{}`, "lbl"); err != nil {
		t.Fatalf("PublishInner: %v", err)
	}
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
	argv := strings.Join(execCall.args, " ")
	if !strings.Contains(argv, wantNS) {
		t.Errorf("exec does not contain discovered namespace %q; argv=%s", wantNS, argv)
	}
	// Verify DefaultNamespace ("dune") does not appear as the -n flag value.
	// The shell payload may reference "/home/dune/..." paths, so we check for the
	// namespace flag pattern specifically rather than bare string containment.
	if strings.Contains(argv, `'-n' '`+DefaultNamespace+`'`) ||
		strings.Contains(argv, `"-n" "`+DefaultNamespace+`"`) ||
		strings.Contains(argv, `-n `+DefaultNamespace+` `) {
		t.Errorf("exec uses hardcoded DefaultNamespace %q as -n value; argv=%s", DefaultNamespace, argv)
	}
}

// --- TokenSource implementations ---

// TestLiteralToken verifies LiteralToken returns the configured value.
func TestLiteralToken(t *testing.T) {
	lt := LiteralToken{Value: "fake-builtin"}
	tok, err := lt.Token(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("LiteralToken.Token: %v", err)
	}
	if tok != "fake-builtin" {
		t.Errorf("got %q, want %q", tok, "fake-builtin")
	}
}

// TestLiteralToken_Empty verifies that an empty value is an error.
func TestLiteralToken_Empty(t *testing.T) {
	lt := LiteralToken{Value: ""}
	_, err := lt.Token(context.Background(), nil, "")
	if err == nil {
		t.Error("expected error for empty LiteralToken, got nil")
	}
}

// TestBGEnvToken_LiteralValue verifies the literal-env path of BGEnvToken.
func TestBGEnvToken_LiteralValue(t *testing.T) {
	rr := &recordingRunner{
		bgdPodStdout: "dune-bgd-deploy-abc123",
		bgdEnvStdout: "fake-service-token",
	}
	bt := BGEnvToken{}
	tok, err := bt.Token(context.Background(), &ssh.Client{Runner: rr}, "vm-a")
	if err != nil {
		t.Fatalf("BGEnvToken.Token: %v", err)
	}
	if tok != "fake-service-token" {
		t.Errorf("got %q, want %q", tok, "fake-service-token")
	}
	// Confirm no log/print of the token — structural check only (no log calls in impl)
}

// TestBGEnvToken_SecretKeyRef verifies the secretKeyRef fallback of BGEnvToken.
// When the literal-env jsonpath returns empty, BGEnvToken must resolve the secret.
func TestBGEnvToken_SecretKeyRef(t *testing.T) {
	// base64("fake-secret-token") = "ZmFrZS1zZWNyZXQtdG9rZW4="
	encodedToken := base64.StdEncoding.EncodeToString([]byte("fake-secret-token"))
	rr := &recordingRunner{
		bgdPodStdout:        "dune-bgd-deploy-abc123",
		bgdEnvStdout:        "", // empty → triggers secretKeyRef path
		secretRefNameStdout: "rmq-game-secret",
		secretRefKeyStdout:  "serviceAuthToken",
		secretStdout:        encodedToken,
	}
	bt := BGEnvToken{}
	tok, err := bt.Token(context.Background(), &ssh.Client{Runner: rr}, "vm-a")
	if err != nil {
		t.Fatalf("BGEnvToken.Token (secretKeyRef): %v", err)
	}
	if tok != "fake-secret-token" {
		t.Errorf("got %q, want %q", tok, "fake-secret-token")
	}
}

// TestBGEnvToken_Empty verifies an error when both literal and secretKeyRef paths return empty.
func TestBGEnvToken_Empty(t *testing.T) {
	rr := &recordingRunner{
		bgdPodStdout: "dune-bgd-deploy-abc123",
		bgdEnvStdout: "",
		secretStdout: "",
	}
	bt := BGEnvToken{}
	_, err := bt.Token(context.Background(), &ssh.Client{Runner: rr}, "vm-a")
	if err == nil {
		t.Error("expected error for empty token from BGEnvToken, got nil")
	}
}

// TestBGEnvToken_NoPod verifies an error when the bgd-deploy pod is not found.
func TestBGEnvToken_NoPod(t *testing.T) {
	rr := &recordingRunner{
		bgdPodStdout: "",
	}
	bt := BGEnvToken{}
	_, err := bt.Token(context.Background(), &ssh.Client{Runner: rr}, "vm-a")
	if err == nil {
		t.Error("expected error for missing bgd-deploy pod, got nil")
	}
}

// TestChainToken_FirstWins verifies ChainToken returns the first non-empty token.
func TestChainToken_FirstWins(t *testing.T) {
	rr := &recordingRunner{
		bgdPodStdout: "dune-bgd-deploy-abc123",
		bgdEnvStdout: "fake-chain-token",
	}
	ct := ChainToken{Sources: []TokenSource{
		BGEnvToken{},
		LiteralToken{Value: BuiltinToken},
	}}
	tok, err := ct.Token(context.Background(), &ssh.Client{Runner: rr}, "vm-a")
	if err != nil {
		t.Fatalf("ChainToken.Token: %v", err)
	}
	if tok != "fake-chain-token" {
		t.Errorf("got %q, want BGEnv result %q", tok, "fake-chain-token")
	}
}

// TestChainToken_FallsThrough verifies ChainToken falls through to the builtin
// when the first source fails.
func TestChainToken_FallsThrough(t *testing.T) {
	// BGEnvToken will fail: no bgd pod found.
	rr := &recordingRunner{
		bgdPodStdout: "",
	}
	ct := ChainToken{Sources: []TokenSource{
		BGEnvToken{},
		LiteralToken{Value: BuiltinToken},
	}}
	tok, err := ct.Token(context.Background(), &ssh.Client{Runner: rr}, "vm-a")
	if err != nil {
		t.Fatalf("ChainToken.Token fallthrough: %v", err)
	}
	if tok != BuiltinToken {
		t.Errorf("got %q, want BuiltinToken %q", tok, BuiltinToken)
	}
}

// TestChainToken_AllFail verifies ChainToken returns an error when all sources fail.
func TestChainToken_AllFail(t *testing.T) {
	ct := ChainToken{Sources: []TokenSource{
		LiteralToken{Value: ""},
	}}
	_, err := ct.Token(context.Background(), nil, "vm-a")
	if err == nil {
		t.Error("expected error when all sources fail, got nil")
	}
}

// TestDefaultTokenChain verifies that a zero-value Publisher (Token==nil)
// uses the BGEnv→builtin chain as its default.
func TestDefaultTokenChain(t *testing.T) {
	rr := &recordingRunner{
		// -A ns/pod format for the mq pod list.
		mqPodListStdout: "funcom-seabass-x/seabass-mq-game-sts-0\n",
		bgdPodStdout:    "dune-bgd-deploy-abc123",
		bgdEnvStdout:    "fake-chain-default-token",
		publishStdout:   "publish=ok",
	}
	pub := &Publisher{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	_, err := pub.PublishInner(context.Background(), "operator", "vm-a",
		"broadcast.notice", "host=vm-a", `{}`, "notice")
	if err != nil {
		t.Fatalf("PublishInner with default chain: %v", err)
	}
	// Verify a bgd-deploy pod lookup happened (BGEnvToken was tried).
	var bgdLookup bool
	for _, c := range rr.calls {
		if strings.Contains(strings.Join(c.args, " "), "bgd") {
			bgdLookup = true
			break
		}
	}
	if !bgdLookup {
		t.Error("default token chain did not attempt BGEnvToken (no bgd-deploy lookup)")
	}
}

// TestFileToken_ExplicitPath verifies that an explicit FileToken uses the provided path.
func TestFileToken_ExplicitPath(t *testing.T) {
	rr := &recordingRunner{
		// -A ns/pod format.
		mqPodListStdout: "funcom-seabass-x/seabass-mq-game-sts-0\n",
		tokenStdout:     "TOK\n",
		publishStdout:   "publish=ok",
	}
	pub := &Publisher{
		SSH:   &ssh.Client{Runner: rr},
		Store: newTempStore(t),
		Token: FileToken{Path: DefaultTokenPath},
	}
	_, err := pub.PublishInner(context.Background(), "operator", "vm-a",
		"broadcast.notice", "host=vm-a", `{}`, "notice")
	if err != nil {
		t.Fatalf("PublishInner with FileToken: %v", err)
	}
	joined := strings.Join(rr.calls[0].args, " ")
	if !strings.Contains(joined, DefaultTokenPath) {
		t.Errorf("call0 does not contain DefaultTokenPath %q: %v", DefaultTokenPath, rr.calls[0].args)
	}
}

// TestFileToken_EmptyToken verifies that an empty token response is an error.
func TestFileToken_EmptyToken(t *testing.T) {
	rr := &recordingRunner{
		tokenStdout: "  \n",
		// -A ns/pod format (only reached if token were valid; listed for completeness).
		mqPodListStdout: "funcom-seabass-x/seabass-mq-game-sts-0\n",
		publishStdout:   "",
	}
	pub := &Publisher{
		SSH:   &ssh.Client{Runner: rr},
		Store: newTempStore(t),
		Token: FileToken{Path: DefaultTokenPath},
	}
	_, err := pub.PublishInner(context.Background(), "operator", "vm-a",
		"broadcast.notice", "host=vm-a", `{}`, "notice")
	if err == nil {
		t.Error("expected error for empty token, got nil")
	}
}

// TestEncodeEnvelope verifies the outer-envelope structure.
func TestEncodeEnvelope(t *testing.T) {
	inner := []byte(`{"ServerCommand":"X"}`)
	b64 := EncodeEnvelope(inner, "TOKEN123")

	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	var outer struct {
		Version        int    `json:"Version"`
		AuthToken      string `json:"AuthToken"`
		MessageContent string `json:"MessageContent"`
	}
	if err := json.Unmarshal(raw, &outer); err != nil {
		t.Fatalf("json: %v", err)
	}
	if outer.Version != 2 {
		t.Errorf("Version=%d, want 2", outer.Version)
	}
	if outer.AuthToken != "TOKEN123" {
		t.Errorf("AuthToken=%q, want TOKEN123", outer.AuthToken)
	}
	if outer.MessageContent != string(inner) {
		t.Errorf("MessageContent=%q, want %q", outer.MessageContent, string(inner))
	}
}

// TestBuildErlangPublishLabelSafety verifies that unsafe labels are
// replaced with the "smgmt" fallback.
func TestBuildErlangPublishLabelSafety(t *testing.T) {
	const safe = "abc"
	const unsafeLabel = "bad label"
	gotSafe := BuildErlangPublish("PAYLOAD", safe)
	gotUnsafe := BuildErlangPublish("PAYLOAD", unsafeLabel)
	if !strings.Contains(gotSafe, "label="+safe) {
		t.Errorf("safe label not propagated: %s", gotSafe)
	}
	if strings.Contains(gotUnsafe, unsafeLabel) {
		t.Errorf("unsafe label leaked into Erlang: %s", gotUnsafe)
	}
	if !strings.Contains(gotUnsafe, "label=smgmt") {
		t.Errorf("unsafe label not replaced with 'smgmt': %s", gotUnsafe)
	}
}
