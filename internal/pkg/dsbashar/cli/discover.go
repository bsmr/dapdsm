package cli

import (
	"context"
	"fmt"
	"io"

	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

// discoverCmd reports the Dune-prepped status + existing BattleGroups, reached
// through whichever runner the global flags selected (jumphost or local).
func discoverCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	r := newKubeRunner(stderr)
	bgs, err := kube.ListBattleGroupNamespaces(ctx, r)
	if err != nil {
		return err
	}
	if len(bgs) == 0 {
		fmt.Fprintln(stdout, "discover: no BattleGroups found (cluster has none yet)")
		return nil
	}
	fmt.Fprintf(stdout, "discover: %d BattleGroup(s):\n", len(bgs))
	for _, ns := range bgs {
		fmt.Fprintf(stdout, "  - %s\n", ns)
	}
	return nil
}
