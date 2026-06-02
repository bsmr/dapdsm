package dbquery

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// levelCandidates is the ordered list of column names probed in
// dune.player_state for a "level" value. The first match wins.
var levelCandidates = []string{
	"level",
	"character_level",
	"player_level",
	"experience_level",
	"current_level",
	"total_level",
}

// Player is one row returned by PlayerSearch.
type Player struct {
	FLSID         string
	CharacterName string
	OnlineStatus  string
	LastSeen      string
	Level         *int
	PartitionID   *int64
}

// Pos is the live position returned by PlayerPosition.
type Pos struct {
	X         float64
	Y         float64
	Z         float64
	Dimension *int
	Partition *int64
	Class     string
}

// probeLevelExpr returns the first level-like column name found in cols as a
// qualified psql expression ("ps.<col>::int"), or "NULL::int" when none match.
func probeLevelExpr(cols []string) string {
	set := make(map[string]struct{}, len(cols))
	for _, c := range cols {
		set[c] = struct{}{}
	}
	for _, candidate := range levelCandidates {
		if _, ok := set[candidate]; ok {
			return "ps." + candidate + "::int"
		}
	}
	return "NULL::int"
}

// buildSearchSQL returns the player search query with {level_expr} substituted.
// Parameters are passed via psql variables: :'q' for the search term,
// :lim for the row limit. This prevents any form of SQL injection through
// the search term.
func buildSearchSQL(levelExpr string) string {
	const tpl = `WITH matches AS (
  SELECT DISTINCT
    COALESCE(acct."user"::text,'') AS fls_id,
    COALESCE(ps.character_name,'') AS character_name,
    COALESCE(ps.online_status::text,'') AS online_status,
    COALESCE(to_char(ps.last_avatar_activity AT TIME ZONE 'UTC','YYYY-MM-DD HH24:MI:SS'),'') AS last_seen,
    {level_expr} AS player_level,
    a.partition_id
  FROM dune.player_state ps
  LEFT JOIN dune.accounts acct          ON acct.id = ps.account_id
  LEFT JOIN dune.encrypted_accounts enc ON enc.id  = ps.account_id
  LEFT JOIN dune.actors a               ON a.id    = ps.player_pawn_id
  WHERE lower(ps.character_name) LIKE lower(:'q')
     OR lower(convert_from(enc.encrypted_funcom_id,'UTF8')) LIKE lower(:'q')
)
SELECT fls_id, character_name, online_status, last_seen, player_level, partition_id
FROM matches WHERE fls_id <> ''
ORDER BY CASE WHEN lower(online_status)='online' THEN 0 ELSE 1 END, last_seen DESC, character_name ASC
LIMIT :lim;`
	return strings.ReplaceAll(tpl, "{level_expr}", levelExpr)
}

// buildPosSQL returns the player position query. The fls_id parameter is
// passed via a psql variable :'fls_id', never interpolated into the SQL body.
func buildPosSQL() string {
	return `SELECT ((a.transform).location).x::float8 AS x,
       ((a.transform).location).y::float8 AS y,
       ((a.transform).location).z::float8 AS z,
       a.dimension_index, a.partition_id, a.class
FROM dune.player_state ps
JOIN dune.actors a      ON a.id = ps.player_pawn_id
JOIN dune.accounts acct ON acct.id = ps.account_id
WHERE acct."user"::text = :'fls_id' LIMIT 1;`
}

// parseSearchRows parses psql -tA -F '|' output for the search query.
// Each line is: fls_id|character_name|online_status|last_seen|player_level|partition_id
// Empty fields for the last two columns → nil pointers.
func parseSearchRows(stdout string) ([]Player, error) {
	var out []Player
	for _, line := range strings.Split(strings.TrimRight(stdout, "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 6)
		if len(parts) != 6 {
			continue
		}
		p := Player{
			FLSID:         parts[0],
			CharacterName: parts[1],
			OnlineStatus:  parts[2],
			LastSeen:      parts[3],
		}
		if parts[4] != "" {
			v, err := strconv.Atoi(parts[4])
			if err == nil {
				p.Level = &v
			}
		}
		if parts[5] != "" {
			v, err := strconv.ParseInt(parts[5], 10, 64)
			if err == nil {
				p.PartitionID = &v
			}
		}
		out = append(out, p)
	}
	return out, nil
}

// parsePosRow parses a single row of psql output for the position query.
// Returns nil when output is empty (player offline / no live pawn).
// Each line: x|y|z|dimension_index|partition_id|class
func parsePosRow(stdout string) (*Pos, error) {
	line := strings.TrimRight(stdout, "\n")
	if line == "" {
		return nil, nil
	}
	parts := strings.SplitN(line, "|", 6)
	if len(parts) != 6 {
		return nil, nil
	}
	x, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return nil, fmt.Errorf("parse pos x: %w", err)
	}
	y, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return nil, fmt.Errorf("parse pos y: %w", err)
	}
	z, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return nil, fmt.Errorf("parse pos z: %w", err)
	}
	pos := &Pos{X: x, Y: y, Z: z, Class: parts[5]}
	if parts[3] != "" {
		v, err := strconv.Atoi(parts[3])
		if err == nil {
			pos.Dimension = &v
		}
	}
	if parts[4] != "" {
		v, err := strconv.ParseInt(parts[4], 10, 64)
		if err == nil {
			pos.Partition = &v
		}
	}
	return pos, nil
}

// probeLevelColumn queries information_schema.columns for dune.player_state
// and returns the appropriate level expression using probeLevelExpr.
func (r *Runner) probeLevelColumn(ctx context.Context, host string) (string, error) {
	const sql = `SELECT column_name
FROM information_schema.columns
WHERE table_schema = 'dune' AND table_name = 'player_state'
ORDER BY ordinal_position;`
	res, err := r.execNoAudit(ctx, host, sql)
	if err != nil {
		return "", fmt.Errorf("probe level column: %w", err)
	}
	var cols []string
	for _, line := range strings.Split(strings.TrimRight(res.Stdout, "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			cols = append(cols, line)
		}
	}
	return probeLevelExpr(cols), nil
}

// execWithVars runs SQL via psql, passing extra -v key=value bindings so that
// user-controlled values never appear in the SQL body.
func (r *Runner) execWithVars(ctx context.Context, host, sql string, vars map[string]string) (*ExecResult, error) {
	t, err := r.discoverDB(ctx, host)
	if err != nil {
		return nil, err
	}
	db := r.Database
	if db == "" {
		db = DefaultDatabase
	}

	// Build psql argv: base flags, then one -v key=value per variable.
	psqlArgs := []string{
		"exec", "-i", "-n", t.Namespace, t.Pod, "--",
		"env", funcomPGOptions,
		"psql",
		"-h", "127.0.0.1",
		"-p", strconv.Itoa(t.Port),
		"-U", t.SuperUser,
		"-d", db,
		"-tA", "-F", "|",
	}
	for k, v := range vars {
		psqlArgs = append(psqlArgs, "-v", k+"="+v)
	}
	psqlArgs = append(psqlArgs, "-f", "-")

	res, err := r.SSH.RunWithStdin(ctx, host, []byte(sql), "kubectl", psqlArgs...)
	if err != nil {
		return nil, fmt.Errorf("psql exec: %w", err)
	}
	if res.ExitCode != 0 {
		msg := strings.TrimSpace(res.Stderr)
		return nil, fmt.Errorf("psql exit %d: %s", res.ExitCode, msg)
	}
	return &ExecResult{Stdout: res.Stdout, Stderr: res.Stderr}, nil
}

// PlayerSearch finds players matching query (a SQL LIKE pattern applied to
// character_name and encrypted_funcom_id) and returns up to limit rows.
// An empty query defaults to "%" (all players). limit is clamped to [1, 200].
// Read-only; no audit entry.
func (r *Runner) PlayerSearch(ctx context.Context, host, query string, limit int) ([]Player, error) {
	if query == "" {
		query = "%"
	}
	if limit <= 0 || limit > 200 {
		limit = 200
	}

	levelExpr, err := r.probeLevelColumn(ctx, host)
	if err != nil {
		return nil, err
	}

	sql := buildSearchSQL(levelExpr)
	res, err := r.execWithVars(ctx, host, sql, map[string]string{
		"q":   query,
		"lim": strconv.Itoa(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("player search: %w", err)
	}
	return parseSearchRows(res.Stdout)
}

// PlayerPosition returns the live world position of the player identified by
// flsID (the Funcom FLS user UUID). Returns nil when the player is offline or
// has no live pawn (i.e. the query returns no rows). Read-only; no audit entry.
func (r *Runner) PlayerPosition(ctx context.Context, host, flsID string) (*Pos, error) {
	sql := buildPosSQL()
	res, err := r.execWithVars(ctx, host, sql, map[string]string{
		"fls_id": flsID,
	})
	if err != nil {
		return nil, fmt.Errorf("player position: %w", err)
	}
	return parsePosRow(res.Stdout)
}
