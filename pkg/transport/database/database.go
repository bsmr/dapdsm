// Package database resolves Postgres connection details for the Funcom
// game database (`dune` schema) and runs targeted SQL probes against it
// via `kubectl exec` against the StatefulSet pod 0. The package owns no
// long-lived connection — every call shells out fresh through kube.Runner.
package database

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	db "go.muehmer.eu/dapdsm/pkg/transport/db"
	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

// Creds carries the values needed to open a psql session against the
// game database from inside the db pod. SuperPassword is the operator
// password (DatabaseDeployment.spec.superPassword) — used for ops that
// must run as the Postgres super-user (DB init, world_partition reads).
// GameUser/GamePassword are the per-app role that the game-server pods
// connect with at runtime; InitGameUser provisions this role.
type Creds struct {
	Pod           string
	Port          int
	SuperUser     string
	SuperPassword string
	GameUser      string
	GamePassword  string
	Database      string
}

// ResolveCreds reads the first DatabaseDeployment in namespace ns and
// derives the StatefulSet pod 0 name plus connection parameters.
func ResolveCreds(ctx context.Context, runner kube.Runner, ns string) (Creds, error) {
	c, err := db.Resolve(ctx, runner, ns)
	if err != nil {
		return Creds{}, err
	}
	return Creds{
		Pod:           c.Pod,
		Port:          c.Port,
		SuperUser:     c.SuperUser,
		SuperPassword: c.SuperPassword,
		GameUser:      c.GameUser,
		GamePassword:  c.GamePassword,
		Database:      c.Database,
	}, nil
}

// InitGameUser provisions the per-app Postgres role and database the
// Funcom database-operator's util-pod requires before it can initialise
// the schema. Idempotent — re-running on an already-initialised BG is
// a no-op except for the password alignment.
//
// The SQL is delivered via psql's stdin (kubectl exec -i …) because the
// \gexec meta-command is a psql client-side feature that only works on
// scripted input — `psql -c` is single-statement only. Values for the
// role/database/passwords are passed through psql's -v flag so quoting
// characters in the passwords cannot break the statement.
func InitGameUser(ctx context.Context, runner kube.Runner, ns string, creds Creds) error {
	if creds.GameUser == "" || creds.GamePassword == "" || creds.Database == "" {
		return fmt.Errorf("InitGameUser: missing GameUser/GamePassword/Database in Creds")
	}
	conn := &db.Conn{
		Creds: db.Creds{Namespace: ns, Pod: creds.Pod, Port: creds.Port, SuperUser: creds.SuperUser, SuperPassword: creds.SuperPassword},
		Exec:  db.NewKubeExecer(runner),
	}
	_, err := conn.Run(ctx, db.Query{
		Database: "postgres",
		Password: true,
		Vars: []db.Var{
			{Key: "ON_ERROR_STOP", Val: "1"},
			{Key: "db_user", Val: creds.GameUser},
			{Key: "db_password", Val: creds.GamePassword},
			{Key: "db_name", Val: creds.Database},
			{Key: "super_user", Val: creds.SuperUser},
			{Key: "super_password", Val: creds.SuperPassword},
		},
		SQL: q("init_game_user"),
	})
	if err != nil {
		return fmt.Errorf("psql InitGameUser: %w", err)
	}
	return nil
}

// validMapName guards the SQL IN-list against injection. Funcom map
// names are alphanumeric + underscore in every observed CR; anything
// outside that set is rejected before reaching psql.
var validMapName = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// LookupPartitionIDs reads dune.world_partition for the requested maps
// and returns map[mapName]partitionID. Any requested map that the DB
// does not return is reported as an error so an enable-set call cannot
// quietly skip a map.
func LookupPartitionIDs(ctx context.Context, runner kube.Runner, ns string, creds Creds, maps []string) (map[string]int, error) {
	for _, m := range maps {
		if !validMapName.MatchString(m) {
			return nil, fmt.Errorf("invalid map name %q (only [A-Za-z0-9_] allowed)", m)
		}
	}
	sql := fmt.Sprintf(
		"SELECT map, partition_id FROM dune.world_partition WHERE map IN ('%s')",
		strings.Join(maps, "','"),
	)
	conn := &db.Conn{
		Creds: db.Creds{Namespace: ns, Pod: creds.Pod, Port: creds.Port, SuperUser: creds.SuperUser, SuperPassword: creds.SuperPassword, Database: creds.Database},
		Exec:  db.NewKubeExecer(runner),
	}
	out, err := conn.Run(ctx, db.Query{Password: true, Tuples: true, Inline: sql})
	if err != nil {
		return nil, fmt.Errorf("psql world_partition lookup: %w", err)
	}
	got := make(map[string]int, len(maps))
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("unexpected psql output line: %q", line)
		}
		id, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("parse partition_id from %q: %w", line, err)
		}
		got[parts[0]] = id
	}
	var missing []string
	for _, m := range maps {
		if _, ok := got[m]; !ok {
			missing = append(missing, m)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("no world_partition row for: %s", strings.Join(missing, ", "))
	}
	return got, nil
}
