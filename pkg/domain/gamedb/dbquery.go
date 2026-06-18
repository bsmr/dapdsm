// Package gamedb runs ad-hoc SQL against the Funcom game database by
// piping it to `psql` inside the DB pod via SSH+kubectl-exec.
package gamedb

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go.muehmer.eu/dapdsm/pkg/domain/store"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

const DefaultDatabase = "dune"

// Runner executes SQL against the Funcom DB pod over SSH+kubectl-exec
// and writes one audit entry per call.
type Runner struct {
	SSH   *ssh.Client
	Store *store.Store
	// Optional overrides. Namespace scopes the DatabaseDeployment lookup
	// (default: cluster-wide, first match). Database overrides the DB name
	// (default: DefaultDatabase). The namespace, pod, port, and superuser
	// are otherwise read from the DatabaseDeployment CR.
	Namespace string
	Database  string
}

// dbTarget is the connection target resolved from the DatabaseDeployment CR.
type dbTarget struct {
	Pod       string // <deployment-name>-sts-0
	Namespace string // metadata.namespace of the DatabaseDeployment
	Port      int    // spec.port (Funcom's PG listens here, not 5432)
	SuperUser string // spec.superUser
}

// discoverDB resolves the DB pod, namespace, port, and superuser from the
// DatabaseDeployment CR in one kubectl get. The pod follows Funcom's
// StatefulSet naming `<deployment-name>-sts-0`. When Runner.Namespace is
// set the lookup is scoped to it, else it is cluster-wide (first match).
func (r *Runner) discoverDB(ctx context.Context, host string) (dbTarget, error) {
	args := []string{"get", "databasedeployment"}
	if r.Namespace != "" {
		args = append(args, "-n", r.Namespace)
	} else {
		args = append(args, "-A")
	}
	args = append(args, "-o",
		"jsonpath={.items[0].metadata.namespace} {.items[0].metadata.name} {.items[0].spec.port} {.items[0].spec.superUser}")

	res, err := r.SSH.Run(ctx, host, "kubectl", args...)
	if err != nil {
		return dbTarget{}, fmt.Errorf("find DatabaseDeployment: %w", err)
	}
	if res.ExitCode != 0 {
		return dbTarget{}, fmt.Errorf("find DatabaseDeployment: kubectl exit %d: %s",
			res.ExitCode, strings.TrimSpace(res.Stderr))
	}

	fields := strings.Fields(strings.TrimSpace(res.Stdout))
	if len(fields) != 4 {
		return dbTarget{}, fmt.Errorf("no DatabaseDeployment found (got %q)", strings.TrimSpace(res.Stdout))
	}
	ns, name, portStr, su := fields[0], fields[1], fields[2], fields[3]

	// Defence-in-depth: the name is interpolated into a kubectl-exec argv
	// before the `--` sentinel; a name starting with `-` would parse as a
	// flag. Cluster-controlled today, but the guard costs nothing.
	if strings.HasPrefix(name, "-") {
		return dbTarget{}, fmt.Errorf("DatabaseDeployment name %q starts with '-'", name)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return dbTarget{}, fmt.Errorf("DatabaseDeployment port %q: %w", portStr, err)
	}
	return dbTarget{Pod: name + "-sts-0", Namespace: ns, Port: port, SuperUser: su}, nil
}
