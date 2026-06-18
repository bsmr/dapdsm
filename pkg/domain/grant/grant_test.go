package grant

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/domain/dbquery"
	"go.muehmer.eu/dapdsm/pkg/domain/mq"
	"go.muehmer.eu/dapdsm/pkg/domain/store"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// recorder is a fake ssh.Runner. Run answers the DatabaseDeployment discovery
// query and the mq-game pod lookup; RunWithStdin answers the is_player_offline
// psql query ("t"/"f"), the UnspentSkillpoints query (unspentOut), the
// GrantTrackXP query (trackXPOut), the GrantCharXP read-state query
// (charXPReadOut), the keystone query (charXPKeystoneOut), and any rabbitmqctl
// publish (capturing stdin in lastMQStdin). It records psql SQL bodies in sqls.
type recorder struct {
	offline           bool
	publishStdout     string
	unspentOut        string // returned for SQL containing UnspentSkillPoints
	trackXPOut        string // returned for SQL containing specialization_tracks
	charXPReadOut     string // returned for SQL containing TotalXPEarned (charxp read-state)
	charXPKeystoneOut string // returned for SQL containing purchased_specialization_keystones
	sqls              []string
	lastMQStdin       []byte // captured stdin from the most recent MQ publish call
}

func (r *recorder) Run(ctx context.Context, name string, args ...string) (ssh.Result, error) {
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "databasedeployment") {
		return ssh.Result{Stdout: "ns-x sh-db 15432 postgres", ExitCode: 0}, nil
	}
	// Satisfy mq.Publisher.discoverGamePod (all-namespace pod list).
	// Return a fake mq-game pod so PublishInner can proceed.
	if strings.Contains(joined, "-A") && strings.Contains(joined, "pods") {
		return ssh.Result{Stdout: "dune/mq-game-fake-0\n", ExitCode: 0}, nil
	}
	return ssh.Result{ExitCode: 0}, nil
}

func (r *recorder) RunWithStdin(ctx context.Context, stdin []byte, name string, args ...string) (ssh.Result, error) {
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "rabbitmqctl") || strings.Contains(string(stdin), "rabbit_queue_type") {
		r.lastMQStdin = append([]byte(nil), stdin...)
		return ssh.Result{Stdout: r.publishStdout, ExitCode: 0}, nil
	}
	r.sqls = append(r.sqls, string(stdin))
	// UnspentSkillpoints query is distinguishable by the jsonb path it reads.
	if strings.Contains(string(stdin), "UnspentSkillPoints") && r.unspentOut != "" {
		return ssh.Result{Stdout: r.unspentOut, ExitCode: 0}, nil
	}
	// GrantCharXP read-state query: contains TotalXPEarned column alias.
	if strings.Contains(string(stdin), "TotalXPEarned") {
		out := r.charXPReadOut
		if out == "" {
			out = "1000|5|5001|6001\n"
		}
		return ssh.Result{Stdout: out, ExitCode: 0}, nil
	}
	// GrantCharXP keystone query: purchased_specialization_keystones (no tracks).
	if strings.Contains(string(stdin), "purchased_specialization_keystones") {
		out := r.charXPKeystoneOut
		return ssh.Result{Stdout: out, ExitCode: 0}, nil
	}
	// GrantTrackXP query is distinguishable by the table it touches.
	if strings.Contains(string(stdin), "specialization_tracks") {
		out := r.trackXPOut
		if out == "" {
			out = "700\n"
		}
		return ssh.Result{Stdout: out, ExitCode: 0}, nil
	}
	if r.offline {
		return ssh.Result{Stdout: "t\n", ExitCode: 0}, nil
	}
	return ssh.Result{Stdout: "f\n", ExitCode: 0}, nil
}

func newTempStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func mustGranter(t *testing.T, offline bool) (*Granter, *recorder) {
	t.Helper()
	rec := &recorder{offline: offline, publishStdout: "publish=ok\n"}
	st := newTempStore(t)
	db := &dbquery.Runner{SSH: &ssh.Client{Runner: rec}, Store: st}
	pub := &mq.Publisher{SSH: &ssh.Client{Runner: rec}, Store: st}
	return &Granter{DB: db, MQ: pub, Store: st}, rec
}

// newTestGranter is the canonical name used by the xp tests; delegates to mustGranter.
func newTestGranter(t *testing.T, offline bool) *Granter {
	t.Helper()
	g, _ := mustGranter(t, offline)
	return g
}

func TestPlanItemOfflineRoutesDB(t *testing.T) {
	g, _ := mustGranter(t, true)
	p, err := g.Plan(context.Background(), "h", Req{Verb: VerbItem, FLS: "ABCD", Item: "Item_Blade", Count: 1})
	if err != nil {
		t.Fatal(err)
	}
	if p.Backend != BackendDB || !p.Offline {
		t.Fatalf("offline item should route DB: %+v", p)
	}
}

func TestPlanItemOnlineRoutesMQ(t *testing.T) {
	g, _ := mustGranter(t, false)
	p, err := g.Plan(context.Background(), "h", Req{Verb: VerbItem, FLS: "ABCD", Item: "Item_Blade", Count: 1})
	if err != nil {
		t.Fatal(err)
	}
	if p.Backend != BackendMQ || p.Offline {
		t.Fatalf("online item should route MQ: %+v", p)
	}
}

func TestPlanCurrencyOnlineWithoutForceRefused(t *testing.T) {
	g, _ := mustGranter(t, false)
	if _, err := g.Plan(context.Background(), "h", Req{Verb: VerbCurrency, FLS: "ABCD", CurrencyID: 1, Delta: 100}); err == nil {
		t.Fatal("online currency without force must be refused")
	}
}

func TestPlanCurrencyOnlineWithForceAllowed(t *testing.T) {
	g, _ := mustGranter(t, false)
	p, err := g.Plan(context.Background(), "h", Req{Verb: VerbCurrency, FLS: "ABCD", CurrencyID: 1, Delta: 100, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if p.Backend != BackendDB {
		t.Fatalf("currency is DB-only: %+v", p)
	}
}

func TestValidationRejectsBadCaps(t *testing.T) {
	g, _ := mustGranter(t, true)
	bad := []Req{
		{Verb: VerbSkillpoints, FLS: "ABCD", Amount: 0},
		{Verb: VerbSkillpoints, FLS: "ABCD", Amount: 1001},
		{Verb: VerbItem, FLS: "ABCD", Item: "x", Count: 0},
		{Verb: VerbItem, FLS: "ABCD", Item: "bad name!", Count: 1},
		{Verb: VerbItem, FLS: "ABCD", Item: "ok", Count: maxItemCount + 1},
	}
	for _, r := range bad {
		if _, err := g.Plan(context.Background(), "h", r); err == nil {
			t.Errorf("expected validation error for %+v", r)
		}
	}
}

// TestValidationAcceptsHighItemCount guards the relaxed give-item cap: a count
// well above the old flat 1000 (e.g. a 25000 Solari stack) must validate, since
// the DB enforces no per-template stack max (see reference: merge_inventory_items
// never clamps). maxItemCount is a self-imposed, tunable blast-radius guard.
func TestValidationAcceptsHighItemCount(t *testing.T) {
	g, _ := mustGranter(t, true)
	if _, err := g.Plan(context.Background(), "h", Req{Verb: VerbItem, FLS: "ABCD", Item: "SolarisCoin", Count: 25000}); err != nil {
		t.Fatalf("a 25000 item count must validate under the relaxed cap, got %v", err)
	}
}

func TestPlanSkillpointsOfflineDB(t *testing.T) {
	g, _ := mustGranter(t /*offline=*/, true)
	p, err := g.Plan(context.Background(), "h", Req{Verb: VerbSkillpoints, FLS: "FLS1", Amount: 5})
	if err != nil {
		t.Fatal(err)
	}
	if p.Backend != BackendDB || !p.Offline {
		t.Fatalf("want offline DB, got %+v", p)
	}
}

func TestPlanSkillpointsOnlineNeedsForce(t *testing.T) {
	g, _ := mustGranter(t /*offline=*/, false)
	if _, err := g.Plan(context.Background(), "h", Req{Verb: VerbSkillpoints, FLS: "FLS1", Amount: 5}); err == nil {
		t.Fatal("online skillpoints without --force must error")
	}
}

func TestPlanSkillpointsOnlineForceMQ(t *testing.T) {
	g, _ := mustGranter(t /*offline=*/, false)
	p, err := g.Plan(context.Background(), "h", Req{Verb: VerbSkillpoints, FLS: "FLS1", Amount: 5, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if p.Backend != BackendMQ {
		t.Fatalf("want MQ backend, got %+v", p)
	}
}

// TestApplySkillpointsOnlineMQ verifies the read-modify-write path: UnspentSkillpoints
// returns base=10; Apply publishes SkillsSetUnspentSkillPoints with SkillPoints=15
// (10 + 5) via PublishInner, and the Result carries the expected detail string.
func TestApplySkillpointsOnlineMQ(t *testing.T) {
	g, rec := mustGranter(t /*offline=*/, false)
	rec.unspentOut = "10\n" // fake UnspentSkillpoints DB read

	res, err := g.Apply(context.Background(), "op", "h",
		Req{Verb: VerbSkillpoints, FLS: "FLS1", Amount: 5, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("expected OK result, got %+v", res)
	}

	// The Erlang expression passed to the MQ pod must contain the base64-encoded
	// envelope whose MessageContent is the skillpoints inner JSON.
	expectedInner := mq.BuildSkillpointsCommand("FLS1", 15)
	expectedB64 := mq.EncodeEnvelope([]byte(expectedInner), mq.BuiltinToken)
	if !strings.Contains(string(rec.lastMQStdin), expectedB64) {
		t.Fatalf("MQ publish stdin does not contain expected base64 envelope\nwant substring: %s\ngot stdin: %s",
			expectedB64, string(rec.lastMQStdin))
	}
}

func TestCanonicalTrack(t *testing.T) {
	if got, ok := CanonicalTrack("combat"); !ok || got != "Combat" {
		t.Fatalf("combat → %q,%v", got, ok)
	}
	if _, ok := CanonicalTrack("nope"); ok {
		t.Fatal("unknown track must not validate")
	}
}

func TestPlanXPOfflineDBOnlineMQ(t *testing.T) {
	off := newTestGranter(t, true)
	p, err := off.Plan(context.Background(), "h", Req{Verb: VerbXP, FLS: "FLS1", Track: "Combat", XP: 500})
	if err != nil {
		t.Fatal(err)
	}
	if p.Backend != BackendDB {
		t.Fatalf("offline want DB, got %v", p.Backend)
	}
	on := newTestGranter(t, false)
	p, err = on.Plan(context.Background(), "h", Req{Verb: VerbXP, FLS: "FLS1", Track: "Combat", XP: 500})
	if err != nil {
		t.Fatalf("online xp must not require --force: %v", err)
	}
	if p.Backend != BackendMQ {
		t.Fatalf("online want MQ, got %v", p.Backend)
	}
}

func TestValidateXPRejects(t *testing.T) {
	g := newTestGranter(t, true)
	if _, err := g.Plan(context.Background(), "h", Req{Verb: VerbXP, FLS: "FLS1", Track: "Nope", XP: 10}); err == nil {
		t.Fatal("unknown track must error")
	}
	if _, err := g.Plan(context.Background(), "h", Req{Verb: VerbXP, FLS: "FLS1", Track: "Combat", XP: 999999}); err == nil {
		t.Fatal("over-cap xp must error")
	}
}

// TestApplyXPOfflineDB verifies the offline path reaches GrantTrackXP and
// returns the new xp_amount from the DB fake.
func TestApplyXPOfflineDB(t *testing.T) {
	g, rec := mustGranter(t, true)
	rec.trackXPOut = "900\n"

	res, err := g.Apply(context.Background(), "op", "h",
		Req{Verb: VerbXP, FLS: "FLS1", Track: "Combat", XP: 200})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("expected OK result, got %+v", res)
	}
	if !strings.Contains(res.Detail, "900") {
		t.Fatalf("detail should contain new xp_amount 900, got %q", res.Detail)
	}
	// Confirm the SQL hit the specialization_tracks table.
	found := false
	for _, s := range rec.sqls {
		if strings.Contains(s, "specialization_tracks") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected GrantTrackXP SQL to contain specialization_tracks")
	}
}

// TestApplyXPOnlineMQ verifies the online path publishes an AwardXP inner JSON
// that contains "AwardXP" and "Category":"Combat".
func TestApplyXPOnlineMQ(t *testing.T) {
	g, rec := mustGranter(t, false)

	res, err := g.Apply(context.Background(), "op", "h",
		Req{Verb: VerbXP, FLS: "FLS1", Track: "Combat", XP: 500})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("expected OK result, got %+v", res)
	}

	expectedInner := mq.BuildAwardXPCommand("FLS1", "Combat", 500)
	expectedB64 := mq.EncodeEnvelope([]byte(expectedInner), mq.BuiltinToken)
	if !strings.Contains(string(rec.lastMQStdin), expectedB64) {
		t.Fatalf("MQ publish stdin does not contain expected base64 envelope\nwant substring: %s\ngot stdin: %s",
			expectedB64, string(rec.lastMQStdin))
	}
}

func TestPlanCharXPOfflineDB(t *testing.T) {
	g := newTestGranter(t, true)
	p, err := g.Plan(context.Background(), "h", Req{Verb: VerbCharXP, FLS: "FLS1", XP: 500})
	if err != nil {
		t.Fatal(err)
	}
	if p.Backend != BackendDB {
		t.Fatalf("want DB, got %v", p.Backend)
	}
}

func TestPlanCharXPOnlineNeedsForce(t *testing.T) {
	g := newTestGranter(t, false)
	if _, err := g.Plan(context.Background(), "h", Req{Verb: VerbCharXP, FLS: "FLS1", XP: 500}); err == nil {
		t.Fatal("online charxp without --force must error")
	}
	p, err := g.Plan(context.Background(), "h", Req{Verb: VerbCharXP, FLS: "FLS1", XP: 500, Force: true})
	if err != nil {
		t.Fatalf("online charxp with --force must plan: %v", err)
	}
	if p.Backend != BackendDB {
		t.Fatalf("charxp with --force must still route BackendDB, got %v", p.Backend)
	}
}

func TestValidateCharXPCapRejected(t *testing.T) {
	g := newTestGranter(t, true)
	bad := []Req{
		{Verb: VerbCharXP, FLS: "FLS1", XP: 0},
		{Verb: VerbCharXP, FLS: "FLS1", XP: -1},
		{Verb: VerbCharXP, FLS: "FLS1", XP: maxCharXP + 1},
	}
	for _, r := range bad {
		if _, err := g.Plan(context.Background(), "h", r); err == nil {
			t.Errorf("expected validation error for XP=%d", r.XP)
		}
	}
}

// TestApplyCharXPOfflineDB verifies the offline path reaches GrantCharXP,
// the recorder feeds back a level-2 result, and Detail mentions the level.
func TestApplyCharXPOfflineDB(t *testing.T) {
	g, rec := mustGranter(t, true)
	// read-state: currentXP=10000|spentSP=0|pawn=5001|controller=6001
	rec.charXPReadOut = "10000|0|5001|6001\n"
	// keystones: empty — no bonus SP
	rec.charXPKeystoneOut = "\n"

	res, err := g.Apply(context.Background(), "op", "h",
		Req{Verb: VerbCharXP, FLS: "FLS1", XP: 500})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("expected OK result, got %+v", res)
	}
	// Detail must mention the level number.
	if !strings.Contains(res.Detail, "level") {
		t.Fatalf("detail should mention level, got %q", res.Detail)
	}
	// SQL must have hit TotalXPEarned (read-state) and TechKnowledgePlayerComponent (apply).
	found := false
	for _, s := range rec.sqls {
		if strings.Contains(s, "TechKnowledgePlayerComponent") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected charxp apply SQL (TechKnowledgePlayerComponent) in recorder.sqls")
	}
}
