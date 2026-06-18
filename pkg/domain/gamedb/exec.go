package gamedb

import (
	"context"
	"sort"

	"go.muehmer.eu/dapdsm/pkg/domain/store"
	db "go.muehmer.eu/dapdsm/pkg/transport/db"
)

// ExecResult captures one psql invocation's outcome. Stdout is the
// raw `-tA -F "|"` output (pipe-separated, one row per line).
type ExecResult struct {
	Stdout string
	Stderr string
}

// Exec runs sql against the Funcom DB pod over SSH+kubectl-exec and
// records an audit entry with Action="db.exec". Use this for any
// operator-initiated query.
func (r *Runner) Exec(ctx context.Context, operator, host, sql string) (*ExecResult, error) {
	audit := store.AuditEntry{
		Operator: operator,
		Host:     host,
		Action:   "db.exec",
		Subject:  truncate(sql, 200),
	}
	res, err := r.execNoAudit(ctx, host, sql)
	switch {
	case err != nil:
		audit.Result = "error: " + err.Error()
	default:
		audit.Result = "ok"
	}
	_ = r.Store.AppendAudit(audit)
	return res, err
}

// execNoAudit is the unaudited wire path. Catalog-browsing helpers
// (Tables, Columns, SlowQueries) use it so a dashboard page-load
// doesn't fill the audit bucket with system reads.
func (r *Runner) execNoAudit(ctx context.Context, host, sql string) (*ExecResult, error) {
	return r.run(ctx, host, sql, nil)
}

// run resolves the DB target for host and executes sql (optionally with psql
// -v vars) through the unified db.Conn over SSH. Trust auth (no PGPASSWORD),
// search_path=dune, tuples-only output — the long-standing query-path wire shape.
func (r *Runner) run(ctx context.Context, host, sql string, vars map[string]string) (*ExecResult, error) {
	t, err := r.discoverDB(ctx, host)
	if err != nil {
		return nil, err
	}
	dbName := r.Database
	if dbName == "" {
		dbName = DefaultDatabase
	}
	conn := &db.Conn{
		Creds: db.Creds{Namespace: t.Namespace, Pod: t.Pod, Port: t.Port, SuperUser: t.SuperUser, Database: dbName},
		Exec:  db.NewSSHExecer(r.SSH, host),
	}
	out, err := conn.Run(ctx, db.Query{SearchPath: true, Tuples: true, Vars: sortedVars(vars), SQL: sql})
	if err != nil {
		return nil, err
	}
	return &ExecResult{Stdout: out}, nil
}

// sortedVars converts the psql var map into a key-ordered []db.Var so the
// generated argv is deterministic.
func sortedVars(vars map[string]string) []db.Var {
	if len(vars) == 0 {
		return nil
	}
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]db.Var, 0, len(keys))
	for _, k := range keys {
		out = append(out, db.Var{Key: k, Val: vars[k]})
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
