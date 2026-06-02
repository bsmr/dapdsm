package grant

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/dbquery"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/mq"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// recorder is a fake ssh.Runner. Run answers the DatabaseDeployment discovery
// query; RunWithStdin answers the is_player_offline psql query ("t"/"f") and any
// rabbitmqctl publish. It records psql SQL bodies in sqls.
type recorder struct {
	offline       bool
	publishStdout string
	sqls          []string
}

func (r *recorder) Run(ctx context.Context, name string, args ...string) (ssh.Result, error) {
	if strings.Contains(strings.Join(args, " "), "databasedeployment") {
		return ssh.Result{Stdout: "ns-x sh-db 15432 postgres", ExitCode: 0}, nil
	}
	return ssh.Result{ExitCode: 0}, nil
}

func (r *recorder) RunWithStdin(ctx context.Context, stdin []byte, name string, args ...string) (ssh.Result, error) {
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "rabbitmqctl") || strings.Contains(string(stdin), "rabbit_queue_type") {
		return ssh.Result{Stdout: r.publishStdout, ExitCode: 0}, nil
	}
	r.sqls = append(r.sqls, string(stdin))
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
		{Verb: VerbItem, FLS: "ABCD", Item: "ok", Count: 1001},
	}
	for _, r := range bad {
		if _, err := g.Plan(context.Background(), "h", r); err == nil {
			t.Errorf("expected validation error for %+v", r)
		}
	}
}
