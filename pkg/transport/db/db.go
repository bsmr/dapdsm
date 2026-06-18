// Package db is the unified Postgres access primitive for the Funcom game
// database. It resolves the DatabaseDeployment, builds one psql argv, and
// runs it in the DB pod via an injected PodExecer (local kubectl or SSH).
// It holds no domain types and does no auditing — callers wrap it.
package db

import (
	"context"
	"fmt"
	"strconv"
)

// PodExecer runs an in-pod command (optional stdin) and returns stdout.
// A non-zero process exit must be returned as an error.
type PodExecer interface {
	Run(ctx context.Context, ns, pod string, stdin []byte, command ...string) (string, error)
}

// Creds are the connection parameters read from the DatabaseDeployment CR.
type Creds struct {
	Namespace     string
	Pod           string
	Port          int
	SuperUser     string
	SuperPassword string
	GameUser      string
	GamePassword  string
	Database      string
}

// Var is one psql -v assignment. A slice (not a map) so callers control order.
type Var struct{ Key, Val string }

// Query is a single psql invocation. Set SQL (piped on stdin via -f -) or
// Inline (-c, single statement) — not both.
type Query struct {
	Database   string // "" => Creds.Database
	User       string // "" => Creds.SuperUser
	Password   bool   // env PGPASSWORD=Creds.SuperPassword
	SearchPath bool   // env PGOPTIONS=-c search_path=dune,public
	Tuples     bool   // -tA -F "|"
	Vars       []Var
	SQL        string
	Inline     string
}

// Conn binds resolved Creds to an execution transport.
type Conn struct {
	Creds Creds
	Exec  PodExecer
}

// Run builds the psql argv from q and runs it in the DB pod. It returns the
// command's stdout; a non-zero psql exit surfaces as an error via the PodExecer.
func (c *Conn) Run(ctx context.Context, q Query) (string, error) {
	db := q.Database
	if db == "" {
		db = c.Creds.Database
	}
	user := q.User
	if user == "" {
		user = c.Creds.SuperUser
	}

	var cmd, env []string
	if q.Password {
		env = append(env, "PGPASSWORD="+c.Creds.SuperPassword)
	}
	if q.SearchPath {
		env = append(env, "PGOPTIONS=-c search_path=dune,public")
	}
	if len(env) > 0 {
		cmd = append(cmd, "env")
		cmd = append(cmd, env...)
	}
	cmd = append(cmd, "psql", "-h", "127.0.0.1", "-p", strconv.Itoa(c.Creds.Port), "-U", user, "-d", db)
	if q.Tuples {
		cmd = append(cmd, "-tA", "-F", "|")
	}
	for _, v := range q.Vars {
		cmd = append(cmd, "-v", v.Key+"="+v.Val)
	}

	var stdin []byte
	if q.Inline != "" {
		cmd = append(cmd, "-c", q.Inline)
	} else {
		cmd = append(cmd, "-f", "-")
		stdin = []byte(q.SQL)
	}
	out, err := c.Exec.Run(ctx, c.Creds.Namespace, c.Creds.Pod, stdin, cmd...)
	if err != nil {
		return "", fmt.Errorf("psql: %w", err)
	}
	return out, nil
}
