package dbquery

import (
	"context"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

func TestSlowQueriesParses(t *testing.T) {
	rr := &pipingRunner{
		podStdout: fakeDBDeploy,
		mqStdout:  "120.5|42|SELECT * FROM dune.player\n12.0|7|UPDATE dune.foo SET x=1\n",
	}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	got, err := r.SlowQueries(context.Background(), "vm-a", 10)
	if err != nil {
		t.Fatalf("SlowQueries: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("rows=%d, want 2", len(got))
	}
	if got[0].MeanMS != 120.5 || got[0].Calls != 42 {
		t.Errorf("row[0]=%+v", got[0])
	}
	if got[0].Query != "SELECT * FROM dune.player" {
		t.Errorf("row[0].Query=%q", got[0].Query)
	}
}

func TestSlowQueriesEmpty(t *testing.T) {
	rr := &pipingRunner{
		podStdout: fakeDBDeploy,
		mqStdout:  "",
	}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	got, err := r.SlowQueries(context.Background(), "vm-a", 10)
	if err != nil {
		t.Fatalf("SlowQueries: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("rows=%d, want 0", len(got))
	}
}
