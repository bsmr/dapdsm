package database

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

// fakeRunner is a Runner that serves canned Get / Exec responses
// keyed by a substring match on the joined argument list. Keeps tests
// readable without growing a real fixture-loader.
type fakeRunner struct {
	getResp     map[string][]byte
	execResp    map[string][]byte
	execErr     map[string]error
	lastExecCmd []string
	// execCapture, if set, is called for every Exec invocation with the
	// full command slice. Used by tests that need to assert on multiple
	// consecutive Exec calls (e.g. multi-statement SQL workflows).
	execCapture func(command ...string)
}

func (f *fakeRunner) Get(_ context.Context, args ...string) ([]byte, error) {
	joined := strings.Join(args, " ")
	for needle, body := range f.getResp {
		if strings.Contains(joined, needle) {
			return body, nil
		}
	}
	return nil, errors.New("fakeRunner.Get: no canned response for " + joined)
}

func (f *fakeRunner) Patch(context.Context, string, string, string, string, string) error {
	return nil
}

func (f *fakeRunner) DeletePods(context.Context, string, ...string) error { return nil }

func (f *fakeRunner) Exec(_ context.Context, _, _ string, command ...string) ([]byte, error) {
	f.lastExecCmd = command
	if f.execCapture != nil {
		f.execCapture(command...)
	}
	joined := strings.Join(command, " ")
	for needle, err := range f.execErr {
		if strings.Contains(joined, needle) {
			return nil, err
		}
	}
	for needle, body := range f.execResp {
		if strings.Contains(joined, needle) {
			return body, nil
		}
	}
	return nil, errors.New("fakeRunner.Exec: no canned response for " + joined)
}

// ExecPiped behaves like Exec for canned-response lookup, but also captures
// the stdin payload so tests can assert on the SQL sent in.
func (f *fakeRunner) ExecPiped(_ context.Context, _, _ string, stdin []byte, command ...string) ([]byte, error) {
	// Treat the stdin payload as an additional "command part" for capture and
	// for substring matching, so tests can pin both the args and the script.
	full := append(append([]string{}, command...), string(stdin))
	f.lastExecCmd = full
	if f.execCapture != nil {
		f.execCapture(full...)
	}
	joined := strings.Join(full, " ")
	for needle, err := range f.execErr {
		if strings.Contains(joined, needle) {
			return nil, err
		}
	}
	for needle, body := range f.execResp {
		if strings.Contains(joined, needle) {
			return body, nil
		}
	}
	return nil, errors.New("fakeRunner.ExecPiped: no canned response for " + joined)
}

// dbdeploymentJSON shapes the minimal subset of a DatabaseDeployment list
// response the resolver needs. user/password are the per-app role that
// InitGameUser provisions; super* is the operator that runs the SQL.
const dbdeploymentJSON = `{
  "items": [
    {
      "metadata": { "name": "sh-3386a7dc456b968d-evjyyk-db-dbdepl" },
      "spec": {
        "port": 15432,
        "superUser": "postgres",
        "superPassword": "KWwHydr21UEMKoloSA3oNuSZ",
        "user": "dune",
        "password": "fVzs021dC8wyuJHcAtqFB6FS",
        "gameDatabaseName": "dune"
      }
    }
  ]
}`

func TestResolveCreds_ParsesDatabaseDeployment(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{getResp: map[string][]byte{"databasedeployment": []byte(dbdeploymentJSON)}}
	got, err := ResolveCreds(context.Background(), r, "funcom-seabass-sh-3386a7dc456b968d-evjyyk")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := Creds{
		Pod:           "sh-3386a7dc456b968d-evjyyk-db-dbdepl-sts-0",
		Port:          15432,
		SuperUser:     "postgres",
		SuperPassword: "KWwHydr21UEMKoloSA3oNuSZ",
		GameUser:      "dune",
		GamePassword:  "fVzs021dC8wyuJHcAtqFB6FS",
		Database:      "dune",
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestInitGameUser_RunsExpectedSQL(t *testing.T) {
	t.Parallel()
	// Capture the SQL passed to psql so we can assert the three required
	// statements appear (CREATE ROLE conditional, ALTER ROLE, CREATE DATABASE
	// conditional, plus the super-password align in a second exec).
	var capturedSQL []string
	r := &fakeRunner{
		execResp: map[string][]byte{"psql": []byte("")},
	}
	// Wrap Exec via wrapper to capture each call's SQL.
	r.execCapture = func(command ...string) {
		// pick out the SQL after "-c" if present, else "-f"; in our case we use stdin
		joined := strings.Join(command, "\x00")
		capturedSQL = append(capturedSQL, joined)
	}
	creds := Creds{
		Pod:           "sh-x-db-dbdepl-sts-0",
		Port:          15432,
		SuperUser:     "postgres",
		SuperPassword: "supersecret",
		GameUser:      "dune",
		GamePassword:  "gamesecret",
		Database:      "dune",
	}
	err := InitGameUser(context.Background(), r, "funcom-seabass-x", creds)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(capturedSQL) == 0 {
		t.Fatalf("no exec calls captured")
	}
	all := strings.Join(capturedSQL, " | ")
	for _, must := range []string{
		"CREATE ROLE",
		"ALTER ROLE",
		"CREATE DATABASE",
		"dune", // both db_user and db_name
	} {
		if !strings.Contains(all, must) {
			t.Errorf("expected SQL to mention %q\n  captured: %s", must, all)
		}
	}
}

func TestInitGameUser_PropagatesPsqlError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("postgres: connection refused")
	r := &fakeRunner{
		execErr: map[string]error{"psql": sentinel},
	}
	creds := Creds{Pod: "x", Port: 15432, SuperUser: "p", SuperPassword: "s", GameUser: "u", GamePassword: "g", Database: "d"}
	err := InitGameUser(context.Background(), r, "ns", creds)
	if err == nil {
		t.Fatalf("err = nil, want propagated psql error")
	}
}

func TestResolveCreds_EmptyItemsErrors(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{getResp: map[string][]byte{"databasedeployment": []byte(`{"items":[]}`)}}
	_, err := ResolveCreds(context.Background(), r, "funcom-seabass-sh-abc")
	if err == nil || !strings.Contains(err.Error(), "no DatabaseDeployment") {
		t.Errorf("err = %v, want substring 'no DatabaseDeployment'", err)
	}
}

func TestLookupPartitionIDs_ReturnsMapAndIssuesExpectedSQL(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{
		execResp: map[string][]byte{
			// pipe-separated, one row per map (matches psql -tA -F'|' output).
			"SELECT map, partition_id FROM dune.world_partition": []byte(
				"Survival_1|1\nSH_Arrakeen|3\nDeepDesert_1|8\n",
			),
		},
	}
	creds := Creds{
		Pod:           "sh-evjyyk-db-dbdepl-sts-0",
		Port:          15432,
		SuperUser:     "postgres",
		SuperPassword: "secret",
		Database:      "dune",
	}
	got, err := LookupPartitionIDs(context.Background(), r, "funcom-seabass-sh-evjyyk", creds,
		[]string{"Survival_1", "SH_Arrakeen", "DeepDesert_1"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := map[string]int{"Survival_1": 1, "SH_Arrakeen": 3, "DeepDesert_1": 8}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("got[%q] = %d, want %d", k, got[k], v)
		}
	}

	// Sanity-check the command shape: tab-/pipe-separated, talks to 127.0.0.1
	// at the resolved port, references the correct database.
	joined := strings.Join(r.lastExecCmd, " ")
	for _, must := range []string{"psql", "-h 127.0.0.1", "-p 15432", "-U postgres", "-d dune", "-tA", "world_partition"} {
		if !strings.Contains(joined, must) {
			t.Errorf("exec cmd missing %q\n  got: %s", must, joined)
		}
	}
}

func TestLookupPartitionIDs_UnknownMapIsError(t *testing.T) {
	t.Parallel()
	// psql returns nothing for unknown maps — caller must error so the
	// always-on flow doesn't quietly skip a map.
	r := &fakeRunner{
		execResp: map[string][]byte{"world_partition": []byte("Survival_1|1\n")},
	}
	creds := Creds{Port: 15432, SuperUser: "u", SuperPassword: "p", Database: "dune", Pod: "x"}
	_, err := LookupPartitionIDs(context.Background(), r, "ns", creds,
		[]string{"Survival_1", "NoSuchMap"})
	if err == nil || !strings.Contains(err.Error(), "NoSuchMap") {
		t.Errorf("err = %v, want substring 'NoSuchMap'", err)
	}
}

// silence unused-import diagnostic when running this file alone.
var _ = kube.Runner(nil)
