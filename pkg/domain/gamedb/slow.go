package gamedb

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// SlowQuery is one row of pg_stat_statements summary.
type SlowQuery struct {
	MeanMS float64
	Calls  int64
	Query  string
}

// SlowQueries reads the top-N entries from pg_stat_statements ordered
// by mean execution time. Returns an empty slice if the extension is
// not installed (Funcom's setup may or may not include it). No audit
// entry — operational metric, not operator action.
func (r *Runner) SlowQueries(ctx context.Context, host string, limit int) ([]SlowQuery, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	sql := fmt.Sprintf(`SELECT mean_exec_time, calls, replace(query, E'\n', ' ')
		FROM pg_stat_statements
		ORDER BY mean_exec_time DESC
		LIMIT %d;`, limit)
	res, err := r.execNoAudit(ctx, host, sql)
	if err != nil {
		return nil, err
	}
	var out []SlowQuery
	for _, line := range strings.Split(strings.TrimRight(res.Stdout, "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}
		mean, _ := strconv.ParseFloat(parts[0], 64)
		calls, _ := strconv.ParseInt(parts[1], 10, 64)
		out = append(out, SlowQuery{MeanMS: mean, Calls: calls, Query: parts[2]})
	}
	return out, nil
}
