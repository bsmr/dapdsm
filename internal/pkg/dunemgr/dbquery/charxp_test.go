package dbquery

import (
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

func TestGrantCharXPMalformedRowErrors(t *testing.T) {
	// 3 fields instead of 4 → shape check error, no mutation attempted.
	rr := &seqRunner{resp: []string{"0|0|5001\n"}}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	if _, err := r.GrantCharXP(context.Background(), "h", "FLS1", 40); err == nil {
		t.Fatal("malformed read row must error")
	}
}

func TestGrantCharXPReadsComputesApplies(t *testing.T) {
	// resp[0] = read state row: currentXP|spentSP|pawn|controller
	// resp[1] = keystone ids (empty → no bonus)
	// resp[2] = apply txn (empty stdout ok)
	rr := &seqRunner{resp: []string{"0|0|5001|6001\n", "\n", "\n"}}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	out, err := r.GrantCharXP(context.Background(), "h", "FLS1", 40)
	if err != nil {
		t.Fatal(err)
	}
	if out.NewLevel != 1 {
		t.Fatalf("level=%d want 1", out.NewLevel)
	}
	all := strings.Join(rr.sqls, "\n")
	for _, want := range []string{"TotalXPEarned", "TotalSkillPoints", "UnspentSkillPoints", "TechKnowledgePlayerComponent", "purchased_specialization_keystones"} {
		if !strings.Contains(all, want) {
			t.Fatalf("charxp SQL missing %q: %q", want, all)
		}
	}
	if strings.Contains(all, "FLS1") {
		t.Fatalf("fls leaked into SQL body: %q", all)
	}
}
