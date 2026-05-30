package dbquery

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// Table identifies one row of pg_catalog.pg_tables / information_schema.tables.
type Table struct {
	Schema string
	Name   string
}

// Column describes one column of an existing table.
type Column struct {
	Name string
	Type string
}

// identRE allows only Postgres-safe identifiers in schema/table args.
// SQL injection via these args is impossible if we never let through
// quotes, semicolons, or other shell-meta.
var identRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)

// Tables lists user tables across the connected database. Reads
// information_schema; no audit entry (catalog browse is framework
// activity, not operator action).
func (r *Runner) Tables(ctx context.Context, host string) ([]Table, error) {
	const sql = `SELECT table_schema, table_name
		FROM information_schema.tables
		WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY table_schema, table_name;`
	res, err := r.execNoAudit(ctx, host, sql)
	if err != nil {
		return nil, err
	}
	var out []Table
	for _, line := range strings.Split(strings.TrimRight(res.Stdout, "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		out = append(out, Table{Schema: parts[0], Name: parts[1]})
	}
	return out, nil
}

// Columns lists columns of one schema.table. No audit entry.
func (r *Runner) Columns(ctx context.Context, host, schema, table string) ([]Column, error) {
	if !identRE.MatchString(schema) {
		return nil, fmt.Errorf("schema %q: invalid identifier", schema)
	}
	if !identRE.MatchString(table) {
		return nil, fmt.Errorf("table %q: invalid identifier", table)
	}
	sql := fmt.Sprintf(`SELECT column_name, data_type
		FROM information_schema.columns
		WHERE table_schema = '%s' AND table_name = '%s'
		ORDER BY ordinal_position;`, schema, table)
	res, err := r.execNoAudit(ctx, host, sql)
	if err != nil {
		return nil, err
	}
	var out []Column
	for _, line := range strings.Split(strings.TrimRight(res.Stdout, "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		out = append(out, Column{Name: parts[0], Type: parts[1]})
	}
	return out, nil
}
