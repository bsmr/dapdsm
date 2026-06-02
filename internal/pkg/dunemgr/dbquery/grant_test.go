package dbquery

import (
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

func TestGrantCurrencyBuildsBoundSQL(t *testing.T) {
	rr := &seqRunner{resp: []string{"1500\n"}}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	bal, err := r.GrantCurrency(context.Background(), "h", "FLS1", 1, 500)
	if err != nil {
		t.Fatal(err)
	}
	if bal != 1500 {
		t.Fatalf("balance=%d want 1500", bal)
	}
	sql := strings.Join(rr.sqls, "\n")
	if !strings.Contains(sql, "adjust_player_virtual_currency_balance") {
		t.Fatalf("missing function call: %q", sql)
	}
	if strings.Contains(sql, "FLS1") {
		t.Fatalf("fls leaked into SQL body: %q", sql)
	}
}

func TestGrantItemDBBuildsBackpackInsert(t *testing.T) {
	rr := &seqRunner{resp: []string{"9001\n"}}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	id, err := r.GrantItemDB(context.Background(), "h", "FLS1", "item.blade", 3, 5)
	if err != nil {
		t.Fatal(err)
	}
	if id != 9001 {
		t.Fatalf("id=%d want 9001", id)
	}
	sql := strings.Join(rr.sqls, "\n")
	for _, want := range []string{"INSERT INTO dune.items", "inventory_type = 0", "generate_series", "RETURNING"} {
		if !strings.Contains(sql, want) {
			t.Fatalf("insert SQL missing %q: %q", want, sql)
		}
	}
	if strings.Contains(sql, "FLS1") {
		t.Fatalf("fls leaked into SQL body: %q", sql)
	}
}

func TestGrantItemDBNoRowMeansFailure(t *testing.T) {
	rr := &seqRunner{resp: []string{"\n"}}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	if _, err := r.GrantItemDB(context.Background(), "h", "FLS1", "item.blade", 1, 0); err == nil {
		t.Fatal("empty result must be an error")
	}
}

func TestGrantSkillpointsBuildsJsonbSet(t *testing.T) {
	rr := &seqRunner{resp: []string{"17\n"}}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	if _, err := r.GrantSkillpoints(context.Background(), "h", "FLS1", 10); err != nil {
		t.Fatal(err)
	}
	sql := strings.Join(rr.sqls, "\n")
	for _, want := range []string{"jsonb_set", "FLevelComponent,1,TotalSkillPoints", "UnspentSkillPoints", "DuneCharacter"} {
		if !strings.Contains(sql, want) {
			t.Fatalf("skillpoints SQL missing %q: %q", want, sql)
		}
	}
	if strings.Contains(sql, "FLS1") {
		t.Fatalf("fls leaked into SQL body: %q", sql)
	}
}
