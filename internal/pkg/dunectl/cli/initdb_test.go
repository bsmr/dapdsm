package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// initDBRunner serves a canned DatabaseDeployment + records psql exec calls.
type initDBRunner struct {
	execCalls int
	execArgs  []string
	execErr   error
}

func (r *initDBRunner) Get(_ context.Context, args ...string) ([]byte, error) {
	if len(args) >= 1 && args[0] == "ns" {
		return []byte("funcom-seabass-sh-deadbeef\n"), nil
	}
	if len(args) >= 1 && args[0] == "databasedeployment" {
		return []byte(`{
  "items": [
    {
      "metadata": {"name": "sh-deadbeef-db-dbdepl"},
      "spec": {
        "port": 15432, "superUser": "postgres", "superPassword": "sup",
        "user": "dune", "password": "gam", "gameDatabaseName": "dune"
      }
    }
  ]
}`), nil
	}
	return nil, errors.New("unexpected Get " + strings.Join(args, " "))
}

func (r *initDBRunner) Patch(context.Context, string, string, string, string, string) error {
	return nil
}

func (r *initDBRunner) DeletePods(context.Context, string, ...string) error { return nil }

func (r *initDBRunner) Exec(_ context.Context, _, _ string, command ...string) ([]byte, error) {
	r.execCalls++
	r.execArgs = command
	return nil, r.execErr
}

func (r *initDBRunner) ExecPiped(_ context.Context, _, _ string, stdin []byte, command ...string) ([]byte, error) {
	r.execCalls++
	// Record the command + the SQL script so tests can assert on both.
	r.execArgs = append(append([]string{}, command...), string(stdin))
	return nil, r.execErr
}

func TestInitDB_CallsPsqlOnce(t *testing.T) {
	t.Parallel()
	r := &initDBRunner{}
	var stdout, stderr bytes.Buffer
	err := runInitDB(context.Background(), nil, &stdout, &stderr, initDBDeps{runner: r})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if r.execCalls != 1 {
		t.Errorf("execCalls = %d, want 1", r.execCalls)
	}
	joined := strings.Join(r.execArgs, " ")
	for _, must := range []string{"psql", "-U postgres", "CREATE ROLE", "db_user=dune"} {
		if !strings.Contains(joined, must) {
			t.Errorf("missing %q in exec args:\n  %s", must, joined)
		}
	}
}

func TestInitDB_PropagatesPsqlError(t *testing.T) {
	t.Parallel()
	r := &initDBRunner{execErr: errors.New("conn refused")}
	var stdout, stderr bytes.Buffer
	err := runInitDB(context.Background(), nil, &stdout, &stderr, initDBDeps{runner: r})
	if err == nil {
		t.Fatalf("err = nil, want propagated error")
	}
}
