package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/dbquery"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// dbCmd runs a DB query (exec|columns|slow) against the BattleGroup database
// on the named host via SSH tunnel.
func dbCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: dunemgr db <host> exec <sql>")
		return fmt.Errorf("db: usage: %w", ErrUsage)
	}
	host, sub, rest := args[0], args[1], args[2:]
	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()
	r := &dbquery.Runner{SSH: ssh.NewClient(), Store: st}
	switch sub {
	case "exec":
		if len(rest) < 1 {
			fmt.Fprintln(stderr, "usage: dunemgr db <host> exec <sql>")
			return fmt.Errorf("db exec: usage: %w", ErrUsage)
		}
		sql := strings.Join(rest, " ")
		res, err := r.Exec(ctx, "cli", host, sql)
		if err != nil {
			return err
		}
		fmt.Fprint(stdout, res.Stdout)
		if res.Stderr != "" {
			fmt.Fprintln(stdout, "--- stderr ---")
			fmt.Fprintln(stdout, res.Stderr)
		}
		return nil
	case "columns":
		if len(rest) < 2 {
			fmt.Fprintln(stderr, "usage: dunemgr db <host> columns <schema> <table>")
			return fmt.Errorf("db columns: usage: %w", ErrUsage)
		}
		cols, err := r.Columns(ctx, host, rest[0], rest[1])
		if err != nil {
			return err
		}
		for _, c := range cols {
			fmt.Fprintf(stdout, "%s\t%s\n", c.Name, c.Type)
		}
		return nil
	case "slow":
		limit := 0
		if len(rest) >= 1 {
			limit, _ = strconv.Atoi(rest[0])
		}
		rows, err := r.SlowQueries(ctx, host, limit)
		if err != nil {
			return err
		}
		for _, q := range rows {
			fmt.Fprintf(stdout, "%.1f\t%d\t%s\n", q.MeanMS, q.Calls, q.Query)
		}
		return nil
	default:
		fmt.Fprintf(stderr, "unknown db subcommand %q (want exec)\n", sub)
		return fmt.Errorf("db: unknown sub %q: %w", sub, ErrUsage)
	}
}
