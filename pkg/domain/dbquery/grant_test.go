package dbquery

import (
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
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

func TestGrantItemDBIgnoresCommandTag(t *testing.T) {
	rr := &seqRunner{resp: []string{"9001\nINSERT 0 1\n"}}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	id, err := r.GrantItemDB(context.Background(), "h", "FLS1", "item.blade", 3, 5)
	if err != nil {
		t.Fatalf("must parse the id line, not choke on the command tag: %v", err)
	}
	if id != 9001 {
		t.Fatalf("id=%d want 9001", id)
	}
}

func TestGrantSkillpointsIgnoresCommandTag(t *testing.T) {
	rr := &seqRunner{resp: []string{"17\nUPDATE 1\n"}}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	n, err := r.GrantSkillpoints(context.Background(), "h", "FLS1", 10)
	if err != nil {
		t.Fatalf("must parse the value line, not choke on the command tag: %v", err)
	}
	if n != 17 {
		t.Fatalf("n=%d want 17", n)
	}
}

func TestGrantSkillpointsAddsUnspentOnly(t *testing.T) {
	rr := &seqRunner{resp: []string{"17\n"}}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	if _, err := r.GrantSkillpoints(context.Background(), "h", "FLS1", 10); err != nil {
		t.Fatal(err)
	}
	sql := strings.Join(rr.sqls, "\n")
	for _, want := range []string{"jsonb_set", "FLevelComponent,1,UnspentSkillPoints", "DuneCharacter", "RETURNING"} {
		if !strings.Contains(sql, want) {
			t.Fatalf("skillpoints SQL missing %q: %q", want, sql)
		}
	}
	if strings.Contains(sql, "TotalSkillPoints") {
		t.Fatalf("Total must no longer be touched: %q", sql)
	}
	if strings.Contains(sql, "FLS1") {
		t.Fatalf("fls leaked into SQL body: %q", sql)
	}
}

func TestUnspentSkillpointsReads(t *testing.T) {
	rr := &seqRunner{resp: []string{"23\n"}}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	n, err := r.UnspentSkillpoints(context.Background(), "h", "FLS1")
	if err != nil {
		t.Fatal(err)
	}
	if n != 23 {
		t.Fatalf("unspent=%d want 23", n)
	}
	sql := strings.Join(rr.sqls, "\n")
	for _, want := range []string{"FLevelComponent,1,UnspentSkillPoints", "DuneCharacter"} {
		if !strings.Contains(sql, want) {
			t.Fatalf("read SQL missing %q: %q", want, sql)
		}
	}
	if strings.Contains(sql, "FLS1") {
		t.Fatalf("fls leaked into SQL body: %q", sql)
	}
}

func TestGrantTrackXPUpsert(t *testing.T) {
	rr := &seqRunner{resp: []string{"3200\n"}}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	n, err := r.GrantTrackXP(context.Background(), "h", "FLS1", "Combat", 200)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3200 {
		t.Fatalf("xp=%d want 3200", n)
	}
	sql := strings.Join(rr.sqls, "\n")
	for _, want := range []string{"dune.specialization_tracks", "track_type", "GREATEST", "LEAST", "44182", "INSERT INTO dune.specialization_tracks"} {
		if !strings.Contains(sql, want) {
			t.Fatalf("track-xp SQL missing %q: %q", want, sql)
		}
	}
	if strings.Contains(sql, "FLS1") || strings.Contains(sql, "Combat") {
		t.Fatalf("fls/track leaked into SQL body: %q", sql)
	}
}
