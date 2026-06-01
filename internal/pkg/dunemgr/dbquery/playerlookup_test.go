package dbquery

import (
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// ---------------------------------------------------------------------------
// Task 3.1: pure SQL builders + parsers (no SSH)
// ---------------------------------------------------------------------------

func TestProbeLevelExprFindsFirstCandidate(t *testing.T) {
	cols := []string{"id", "character_name", "character_level", "account_id"}
	got := probeLevelExpr(cols)
	if got != "ps.character_level::int" {
		t.Errorf("probeLevelExpr=%q, want %q", got, "ps.character_level::int")
	}
}

func TestProbeLevelExprPrefersEarlierCandidate(t *testing.T) {
	// "level" is first in the candidate list and is present.
	cols := []string{"character_level", "level", "experience_level"}
	got := probeLevelExpr(cols)
	if got != "ps.level::int" {
		t.Errorf("probeLevelExpr=%q, want %q", got, "ps.level::int")
	}
}

func TestProbeLevelExprNoneFound(t *testing.T) {
	cols := []string{"id", "account_id", "character_name"}
	got := probeLevelExpr(cols)
	if got != "NULL::int" {
		t.Errorf("probeLevelExpr=%q, want %q", got, "NULL::int")
	}
}

func TestBuildSearchSQLContainsLevelExpr(t *testing.T) {
	sql := buildSearchSQL("ps.character_level::int")
	if !strings.Contains(sql, "ps.character_level::int") {
		t.Errorf("buildSearchSQL missing level expr: %q", sql)
	}
	// Must not contain unsubstituted placeholder.
	if strings.Contains(sql, "{level_expr}") {
		t.Errorf("buildSearchSQL has unsubstituted placeholder: %q", sql)
	}
}

func TestBuildSearchSQLContainsDollarSignFree(t *testing.T) {
	// Parameters must use psql :variable syntax, not $1/$2 (which would
	// require prepared-statement protocol; psql -v binds them as literals).
	sql := buildSearchSQL("NULL::int")
	if strings.Contains(sql, "$1") || strings.Contains(sql, "$2") {
		t.Errorf("buildSearchSQL still uses $1/$2 placeholders (injection risk): %q", sql)
	}
}

func TestBuildPosSQLContainsDollarSignFree(t *testing.T) {
	sql := buildPosSQL()
	if strings.Contains(sql, "$1") {
		t.Errorf("buildPosSQL still uses $1 placeholder (injection risk): %q", sql)
	}
}

func TestBuildSearchSQLUsesVariables(t *testing.T) {
	sql := buildSearchSQL("NULL::int")
	// Expect psql variable references :'q' (quoted) and :lim.
	if !strings.Contains(sql, ":'q'") {
		t.Errorf("buildSearchSQL missing :'q' reference: %q", sql)
	}
	if !strings.Contains(sql, ":lim") {
		t.Errorf("buildSearchSQL missing :lim reference: %q", sql)
	}
}

func TestBuildPosSQLUsesVariable(t *testing.T) {
	sql := buildPosSQL()
	if !strings.Contains(sql, ":'fls_id'") {
		t.Errorf("buildPosSQL missing :'fls_id' reference: %q", sql)
	}
}

// ---------------------------------------------------------------------------
// parseSearchRows tests
// ---------------------------------------------------------------------------

func TestParseSearchRowsHappyPath(t *testing.T) {
	// fls_id|character_name|online_status|last_seen|player_level|partition_id
	input := "fls-abc123|HeroOne|online|2026-05-31 12:00:00|42|7\n"
	players, err := parseSearchRows(input)
	if err != nil {
		t.Fatalf("parseSearchRows: %v", err)
	}
	if len(players) != 1 {
		t.Fatalf("rows=%d, want 1", len(players))
	}
	p := players[0]
	if p.FLSID != "fls-abc123" {
		t.Errorf("FLSID=%q", p.FLSID)
	}
	if p.CharacterName != "HeroOne" {
		t.Errorf("CharacterName=%q", p.CharacterName)
	}
	if p.OnlineStatus != "online" {
		t.Errorf("OnlineStatus=%q", p.OnlineStatus)
	}
	if p.LastSeen != "2026-05-31 12:00:00" {
		t.Errorf("LastSeen=%q", p.LastSeen)
	}
	if p.Level == nil || *p.Level != 42 {
		t.Errorf("Level=%v", p.Level)
	}
	if p.PartitionID == nil || *p.PartitionID != 7 {
		t.Errorf("PartitionID=%v", p.PartitionID)
	}
}

func TestParseSearchRowsEmptyOutput(t *testing.T) {
	players, err := parseSearchRows("")
	if err != nil {
		t.Fatalf("parseSearchRows empty: %v", err)
	}
	if len(players) != 0 {
		t.Errorf("rows=%d, want 0", len(players))
	}
}

func TestParseSearchRowsNullableFieldsNil(t *testing.T) {
	// level and partition_id are empty → pointers must be nil.
	input := "fls-abc123|HeroOne|offline|2026-05-30 10:00:00||\n"
	players, err := parseSearchRows(input)
	if err != nil {
		t.Fatalf("parseSearchRows: %v", err)
	}
	if len(players) != 1 {
		t.Fatalf("rows=%d, want 1", len(players))
	}
	if players[0].Level != nil {
		t.Errorf("Level=%v, want nil", players[0].Level)
	}
	if players[0].PartitionID != nil {
		t.Errorf("PartitionID=%v, want nil", players[0].PartitionID)
	}
}

func TestParseSearchRowsMultipleRows(t *testing.T) {
	input := "a|Alpha|online|2026-05-31 00:00:00||1\nb|Beta|offline|2026-05-30 00:00:00|5|\n"
	players, err := parseSearchRows(input)
	if err != nil {
		t.Fatalf("parseSearchRows: %v", err)
	}
	if len(players) != 2 {
		t.Fatalf("rows=%d, want 2", len(players))
	}
	if players[0].FLSID != "a" || players[1].FLSID != "b" {
		t.Errorf("FLSID order wrong: %+v", players)
	}
}

// ---------------------------------------------------------------------------
// parsePosRow tests
// ---------------------------------------------------------------------------

func TestParsePosRowHappyPath(t *testing.T) {
	// x|y|z|dimension|partition_id|class
	input := "1234.5|6789.0|-42.3|0|3|BP_SomeCharacter_C\n"
	pos, err := parsePosRow(input)
	if err != nil {
		t.Fatalf("parsePosRow: %v", err)
	}
	if pos == nil {
		t.Fatal("pos=nil, want non-nil")
	}
	if pos.X != 1234.5 || pos.Y != 6789.0 || pos.Z != -42.3 {
		t.Errorf("xyz=(%v,%v,%v)", pos.X, pos.Y, pos.Z)
	}
	if pos.Dimension == nil || *pos.Dimension != 0 {
		t.Errorf("Dimension=%v", pos.Dimension)
	}
	if pos.Partition == nil || *pos.Partition != 3 {
		t.Errorf("Partition=%v", pos.Partition)
	}
	if pos.Class != "BP_SomeCharacter_C" {
		t.Errorf("Class=%q", pos.Class)
	}
}

func TestParsePosRowEmptyOutputNil(t *testing.T) {
	pos, err := parsePosRow("")
	if err != nil {
		t.Fatalf("parsePosRow empty: %v", err)
	}
	if pos != nil {
		t.Errorf("pos=%+v, want nil", pos)
	}
}

func TestParsePosRowNullableFieldsNil(t *testing.T) {
	// dimension and partition_id empty → pointers must be nil.
	input := "100.0|200.0|300.0|||SomeClass\n"
	pos, err := parsePosRow(input)
	if err != nil {
		t.Fatalf("parsePosRow: %v", err)
	}
	if pos == nil {
		t.Fatal("pos=nil")
	}
	if pos.Dimension != nil {
		t.Errorf("Dimension=%v, want nil", pos.Dimension)
	}
	if pos.Partition != nil {
		t.Errorf("Partition=%v, want nil", pos.Partition)
	}
}

// ---------------------------------------------------------------------------
// Task 3.2: Runner.PlayerSearch + Runner.PlayerPosition (with fake SSH)
// ---------------------------------------------------------------------------

// searchCapturingRunner captures what psql -v args are passed to confirm
// injection safety, and returns canned results.
type searchCapturingRunner struct {
	podStdout string // kubectl get databasedeployment output
	// two-call sequence: first call = information_schema probe,
	// second call = actual search query
	calls     []capturedCall
	responses []string // canned responses per call index
}

type capturedCall struct {
	stdinSQL string
	args     []string
}

func (r *searchCapturingRunner) Run(ctx context.Context, name string, args ...string) (ssh.Result, error) {
	joined := strings.Join(args, " ")
	// After the shell-quoting fix each remote word is individually quoted;
	// match both the raw and the quoted forms.
	if strings.Contains(joined, "get databasedeployment") ||
		strings.Contains(joined, "'get' 'databasedeployment'") {
		return ssh.Result{Stdout: r.podStdout, ExitCode: 0}, nil
	}
	return ssh.Result{ExitCode: 0}, nil
}

func (r *searchCapturingRunner) RunWithStdin(ctx context.Context, stdin []byte, name string, args ...string) (ssh.Result, error) {
	idx := len(r.calls)
	r.calls = append(r.calls, capturedCall{stdinSQL: string(stdin), args: append([]string{name}, args...)})
	if idx < len(r.responses) {
		return ssh.Result{Stdout: r.responses[idx], ExitCode: 0}, nil
	}
	return ssh.Result{Stdout: "", ExitCode: 0}, nil
}

func TestPlayerSearchReturnsResults(t *testing.T) {
	rr := &searchCapturingRunner{
		podStdout: fakeDBDeploy,
		// call 0: information_schema probe → empty (no level columns present)
		// call 1: search query → one row
		responses: []string{
			"",
			"fls-abc123|HeroOne|online|2026-05-31 12:00:00||7\n",
		},
	}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	players, err := r.PlayerSearch(context.Background(), "vm-a", "Hero%", 10)
	if err != nil {
		t.Fatalf("PlayerSearch: %v", err)
	}
	if len(players) != 1 {
		t.Fatalf("players=%d, want 1", len(players))
	}
	if players[0].FLSID != "fls-abc123" {
		t.Errorf("FLSID=%q", players[0].FLSID)
	}
}

func TestPlayerSearchPassesQueryViaVariable(t *testing.T) {
	// Confirms the search term is passed as a -v variable, not interpolated
	// into the SQL body (injection safety).
	rr := &searchCapturingRunner{
		podStdout: fakeDBDeploy,
		responses: []string{"", ""},
	}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	_, _ = r.PlayerSearch(context.Background(), "vm-a", "'; DROP TABLE dune.player; --", 5)

	// There must be at least 2 calls: probe + search.
	if len(rr.calls) < 2 {
		t.Fatalf("calls=%d, want >=2", len(rr.calls))
	}
	// The malicious string must NOT appear in the SQL body passed to psql.
	searchSQL := rr.calls[1].stdinSQL
	if strings.Contains(searchSQL, "DROP TABLE") {
		t.Errorf("injection: malicious string in SQL body: %q", searchSQL)
	}
	// The search query is passed as a -v argument to psql, not in stdin SQL.
	// After the shell-quoting fix each remote word is individually quoted,
	// so "-v" appears as "'-v'" in the joined args string.
	argsStr := strings.Join(rr.calls[1].args, " ")
	if !strings.Contains(argsStr, "'-v'") && !strings.Contains(argsStr, "-v") {
		t.Errorf("no -v flag in psql args: %q", argsStr)
	}
}

func TestPlayerSearchEmptyQueryDefaultsToAll(t *testing.T) {
	rr := &searchCapturingRunner{
		podStdout: fakeDBDeploy,
		responses: []string{"", ""},
	}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	_, _ = r.PlayerSearch(context.Background(), "vm-a", "", 10)

	if len(rr.calls) < 2 {
		t.Fatalf("calls=%d, want >=2", len(rr.calls))
	}
	// The -v q=... argument must contain '%' (match-all).
	// After the shell-quoting fix, q=% appears as e.g. 'q=%' in the remote token.
	argsStr := strings.Join(rr.calls[1].args, " ")
	if !strings.Contains(argsStr, "q=%") {
		t.Errorf("empty query did not default to '%%': args=%q", argsStr)
	}
	// Both the quoted and unquoted forms are acceptable; the above covers both.
}

func TestPlayerPositionReturnsPos(t *testing.T) {
	rr := &searchCapturingRunner{
		podStdout: fakeDBDeploy,
		responses: []string{"1234.5|6789.0|-42.3|0|3|BP_SomeCharacter_C\n"},
	}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	pos, err := r.PlayerPosition(context.Background(), "vm-a", "fls-abc123")
	if err != nil {
		t.Fatalf("PlayerPosition: %v", err)
	}
	if pos == nil {
		t.Fatal("pos=nil, want non-nil")
	}
	if pos.X != 1234.5 {
		t.Errorf("X=%v", pos.X)
	}
}

func TestPlayerPositionOfflineNil(t *testing.T) {
	rr := &searchCapturingRunner{
		podStdout: fakeDBDeploy,
		responses: []string{""},
	}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	pos, err := r.PlayerPosition(context.Background(), "vm-a", "fls-abc123")
	if err != nil {
		t.Fatalf("PlayerPosition: %v", err)
	}
	if pos != nil {
		t.Errorf("pos=%+v, want nil (offline)", pos)
	}
}

func TestPlayerPositionPassesFLSIDViaVariable(t *testing.T) {
	// Confirms fls_id is passed as a -v variable, not interpolated into SQL.
	rr := &searchCapturingRunner{
		podStdout: fakeDBDeploy,
		responses: []string{""},
	}
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	_, _ = r.PlayerPosition(context.Background(), "vm-a", "'; DROP TABLE dune.player; --")

	if len(rr.calls) < 1 {
		t.Fatalf("calls=%d, want >=1", len(rr.calls))
	}
	posSQL := rr.calls[0].stdinSQL
	if strings.Contains(posSQL, "DROP TABLE") {
		t.Errorf("injection: malicious fls_id in SQL body: %q", posSQL)
	}
	// After the shell-quoting fix "-v" appears as "'-v'" in the remote token.
	argsStr := strings.Join(rr.calls[0].args, " ")
	if !strings.Contains(argsStr, "'-v'") && !strings.Contains(argsStr, "-v") {
		t.Errorf("no -v flag in psql args for PlayerPosition: %q", argsStr)
	}
}
