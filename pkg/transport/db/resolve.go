package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

// Resolve reads the first DatabaseDeployment (scoped to namespace, or
// cluster-wide when namespace is "") and fills Creds. The DB pod follows
// Funcom's StatefulSet naming `<deployment-name>-sts-0`.
func Resolve(ctx context.Context, g kube.Getter, namespace string) (Creds, error) {
	args := []string{"databasedeployment"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	} else {
		args = append(args, "-A")
	}
	args = append(args, "-o", "json")

	raw, err := g.Get(ctx, args...)
	if err != nil {
		return Creds{}, fmt.Errorf("get DatabaseDeployment: %w", err)
	}
	var doc struct {
		Items []struct {
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
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
		return Creds{}, fmt.Errorf("no DatabaseDeployment found (namespace %q)", namespace)
	}
	it := doc.Items[0]
	// Defence-in-depth: the name is interpolated into a kubectl-exec argv
	// before the `--` sentinel; a name starting with `-` would parse as a flag.
	if strings.HasPrefix(it.Metadata.Name, "-") {
		return Creds{}, fmt.Errorf("DatabaseDeployment name %q starts with '-'", it.Metadata.Name)
	}
	return Creds{
		Namespace:     it.Metadata.Namespace,
		Pod:           it.Metadata.Name + "-sts-0",
		Port:          it.Spec.Port,
		SuperUser:     it.Spec.SuperUser,
		SuperPassword: it.Spec.SuperPassword,
		GameUser:      it.Spec.User,
		GamePassword:  it.Spec.Password,
		Database:      it.Spec.GameDatabaseName,
	}, nil
}
