package grant

import (
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/domain/dbquery"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

func TestPersonaHexIDKnown(t *testing.T) {
	if PersonaHexID("GM") == "" || PersonaHexID("Server") == "" {
		t.Fatal("GM/Server personas must have reserved hex ids")
	}
	if PersonaHexID("bogus") != "" {
		t.Fatal("unknown persona must return empty")
	}
}

func TestSeedPersonaWritesBaseTables(t *testing.T) {
	rec := &recorder{offline: true, publishStdout: ""}
	db := &dbquery.Runner{SSH: &ssh.Client{Runner: rec}, Store: newTempStore(t)}
	p := &Persona{DB: db, Store: db.Store}
	if err := p.Seed(context.Background(), "op", "h", "GM"); err != nil {
		t.Fatal(err)
	}
	sql := strings.Join(rec.sqls, "\n")
	for _, want := range []string{
		"INSERT INTO dune.encrypted_accounts",
		"INSERT INTO dune.actors",
		"dune.encrypt_user_data",
		"BEGIN",
		"COMMIT",
		"ON CONFLICT DO NOTHING",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("seed SQL missing %q:\n%s", want, sql)
		}
	}
}
