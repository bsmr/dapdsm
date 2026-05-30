package dbquery

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
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
	t, err := r.discoverDB(ctx, host)
	if err != nil {
		return nil, err
	}
	db := r.Database
	if db == "" {
		db = DefaultDatabase
	}

	res, err := r.SSH.RunWithStdin(ctx, host, []byte(sql),
		"kubectl", "exec", "-i", "-n", t.Namespace, t.Pod, "--",
		"psql",
		"-h", "127.0.0.1",
		"-p", strconv.Itoa(t.Port),
		"-U", t.SuperUser,
		"-d", db,
		"-tA", "-F", "|",
		"-f", "-",
	)
	if err != nil {
		return nil, fmt.Errorf("psql exec: %w", err)
	}
	if res.ExitCode != 0 {
		msg := strings.TrimSpace(res.Stderr)
		return nil, fmt.Errorf("psql exit %d: %s", res.ExitCode, msg)
	}
	return &ExecResult{Stdout: res.Stdout, Stderr: res.Stderr}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
