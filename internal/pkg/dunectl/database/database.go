// Package database resolves Postgres connection details for the Funcom
// game database (`dune` schema) and runs targeted SQL probes against it
// via `kubectl exec` against the StatefulSet pod 0. The package owns no
// long-lived connection — every call shells out fresh through kube.Runner.
package database

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.muehmer.eu/dapdsm/internal/pkg/kube"
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
	raw, err := runner.Get(ctx, "databasedeployment", "-n", ns, "-o", "json")
	if err != nil {
		return Creds{}, fmt.Errorf("get DatabaseDeployment: %w", err)
	}
	var doc struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec struct {
				Port             int    `json:"port"`
				SuperUser        string `json:"superUser"`
				SuperPassword    string `json:"superPassword"`
				User             string `json:"user"`
				Password         string `json:"password"`
				GameDatabaseName string `json:"gameDatabaseName"`
			} `json:"spec"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Creds{}, fmt.Errorf("decode DatabaseDeployment: %w", err)
	}
	if len(doc.Items) == 0 {
		return Creds{}, fmt.Errorf("no DatabaseDeployment in namespace %s", ns)
	}
	it := doc.Items[0]
	return Creds{
		Pod:           it.Metadata.Name + "-sts-0",
		Port:          it.Spec.Port,
		SuperUser:     it.Spec.SuperUser,
		SuperPassword: it.Spec.SuperPassword,
		GameUser:      it.Spec.User,
		GamePassword:  it.Spec.Password,
		Database:      it.Spec.GameDatabaseName,
	}, nil
}

// initGameUserSQL is the idempotent block that creates the per-app
// Postgres role and database the Funcom database-operator's util-pod
// expects to exist before it can initialise the schema. Aligns the
// super-user password too, in case it was rotated out of band.
// Variables are bound via psql's -v flag rather than string interpolation
// so values containing quote characters do not break the statement.
const initGameUserSQL = `SELECT format('CREATE ROLE %I LOGIN PASSWORD %L', :'db_user', :'db_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'db_user') \gexec
ALTER ROLE :"db_user" WITH LOGIN PASSWORD :'db_password';
SELECT format('CREATE DATABASE %I OWNER %I', :'db_name', :'db_user')
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = :'db_name') \gexec
ALTER ROLE :"super_user" WITH PASSWORD :'super_password';
`

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
	_, err := runner.ExecPiped(ctx, ns, creds.Pod, []byte(initGameUserSQL),
		"env", "PGPASSWORD="+creds.SuperPassword,
		"psql",
		"-h", "127.0.0.1",
		"-p", strconv.Itoa(creds.Port),
		"-U", creds.SuperUser,
		"-d", "postgres",
		"-v", "ON_ERROR_STOP=1",
		"-v", "db_user="+creds.GameUser,
		"-v", "db_password="+creds.GamePassword,
		"-v", "db_name="+creds.Database,
		"-v", "super_user="+creds.SuperUser,
		"-v", "super_password="+creds.SuperPassword,
	)
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
	out, err := runner.Exec(ctx, ns, creds.Pod,
		"env", "PGPASSWORD="+creds.SuperPassword,
		"psql",
		"-h", "127.0.0.1",
		"-p", strconv.Itoa(creds.Port),
		"-U", creds.SuperUser,
		"-d", creds.Database,
		"-tA", "-F", "|",
		"-c", sql,
	)
	if err != nil {
		return nil, fmt.Errorf("psql world_partition lookup: %w", err)
	}
	got := make(map[string]int, len(maps))
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
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
