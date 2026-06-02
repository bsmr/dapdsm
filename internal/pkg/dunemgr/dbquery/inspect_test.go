package dbquery

import (
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// seqRunner returns canned RunWithStdin stdouts in order; Run answers discoverDB.
type seqRunner struct {
	resp []string
	n    int
	sqls []string
}

func (r *seqRunner) Run(ctx context.Context, name string, args ...string) (ssh.Result, error) {
	return ssh.Result{Stdout: fakeDBDeploy, ExitCode: 0}, nil
}
func (r *seqRunner) RunWithStdin(ctx context.Context, stdin []byte, name string, args ...string) (ssh.Result, error) {
	r.sqls = append(r.sqls, string(stdin))
	out := ""
	if r.n < len(r.resp) {
		out = r.resp[r.n]
	}
	r.n++
	return ssh.Result{Stdout: out, ExitCode: 0}, nil
}

func TestPlayerInspectParses(t *testing.T) {
	rr := &seqRunner{resp: []string{
		"Stilgar|Offline|2026-06-01 21:40:55|2\n",
		"45|320\n",
		"0|40\n1|5\n",
		"item.water|10|3\nitem.blade|1|5\n",
		"", // components: empty → no Progression/Vitals/Spice populated
	}}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	d, err := r.PlayerInspect(context.Background(), "h", "FLS1", 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Found || d.CharacterName != "Stilgar" || d.OnlineStatus != "Offline" {
		t.Fatalf("header wrong: %+v", d)
	}
	if d.ItemCount != 45 || d.StackTotal != 320 {
		t.Fatalf("totals wrong: %d/%d", d.ItemCount, d.StackTotal)
	}
	if len(d.Inventories) != 2 || d.Inventories[0].ItemCount != 40 {
		t.Fatalf("breakdown wrong: %+v", d.Inventories)
	}
	if len(d.TopItems) != 2 || d.TopItems[1].Quality != 5 {
		t.Fatalf("top items wrong: %+v", d.TopItems)
	}
	for _, s := range rr.sqls {
		if strings.Contains(s, "FLS1") {
			t.Fatalf("fls leaked into SQL body: %q", s)
		}
	}
}

func TestPlayerInspectNotFound(t *testing.T) {
	rr := &seqRunner{resp: []string{"\n"}}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	d, err := r.PlayerInspect(context.Background(), "h", "NOPE", 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if d.Found {
		t.Fatal("want Found=false for unknown fls")
	}
}

func TestPlayerInspectParsesComponents(t *testing.T) {
	rr := &seqRunner{resp: []string{
		"Stilgar|Offline|2026-06-01 21:40:55|2\n", // header
		"45|320\n",               // totals
		"0|40\n",                 // breakdown
		"item.blade|1|5\n",       // top items
		fixtureComponents + "\n", // components (single jsonb line)
	}}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	d, err := r.PlayerInspect(context.Background(), "h", "FLS1", 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if d.Progression == nil || d.Progression.UnspentSkillPoints != 7 {
		t.Fatalf("progression wrong: %+v", d.Progression)
	}
	if d.Vitals == nil || d.Vitals.CurrentHealth != 150.0 {
		t.Fatalf("vitals wrong: %+v", d.Vitals)
	}
	if d.Spice == nil || d.Spice.SystemStatus != "AddictionDisabled" {
		t.Fatalf("spice wrong: %+v", d.Spice)
	}
	if d.RawComponents != "" {
		t.Fatalf("raw should be empty when raw=false, got %q", d.RawComponents)
	}
	for _, s := range rr.sqls {
		if strings.Contains(s, "FLS1") {
			t.Fatalf("fls leaked into SQL body: %q", s)
		}
	}
}

func TestPlayerInspectRawPopulatesComponents(t *testing.T) {
	rr := &seqRunner{resp: []string{
		"Stilgar|Offline|2026-06-01 21:40:55|2\n", "45|320\n", "0|40\n", "item.blade|1|5\n",
		fixtureComponents + "\n",
	}}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	d, err := r.PlayerInspect(context.Background(), "h", "FLS1", 10, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.RawComponents, "FLevelComponent") {
		t.Fatalf("raw components not populated: %q", d.RawComponents)
	}
}
